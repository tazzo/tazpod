"""
paperclip-egress — the mitmproxy addon behind the container's egress chokepoint (design T4b).

Every request the agent makes arrives here first, because (a) the agent's environment points at
this proxy and (b) the packet filter in the same container leaves the agent's uid no other way
out. This addon answers three questions, in this order:

  1. **May this destination be reached at all?** The policy is an allowlist; anything not in it
     gets a 403 and a journal line. Enforced in `http_connect` (before any TLS is touched, so a
     refused destination is refused without us talking to it) *and* again in `request` (so a
     plain-HTTP absolute-URI request cannot dodge it).
  2. **Is the destination the one the CONNECT asked for?** A mismatch between the intercepted
     connection's authority and the inner request's Host is domain fronting: refused and logged.
  3. **Does this request need a credential?** If the matching rule brokers one, the credential is
     added here as a header, and the client's own version of that header is stripped, so what
     reaches the target is the brokered value and never the agent's.

It also *is* the Vault AppRole client for this container. The SecretID T4 mints is single-use and
response-wrapped; the agent cannot log in with it (and must not), so the login happens here: at
startup the addon unwraps the delivered wrapping token, exchanges it for an AppRole client token,
keeps that token **in memory only**, renews it on the token's period, and injects it as
`X-Vault-Token` on requests to the Vault address. The agent therefore calls Vault through the
chokepoint and never holds a Vault credential of any kind.

What it deliberately does *not* have:

  * no `gopass`, no `gpg-agent`, no pinentry: the credentials are 0600 files read at startup, and
    the Vault token never touches the disk at all.
  * no JSON-body injection. The Vault rule used to inject the SecretID into the AppRole login
    body; now that the proxy performs the login itself there is no target left that needs a body
    field, so the machinery is gone rather than left as dead code.
  * no notion of "who" the caller is beyond the client address. Over TCP the peer is always
    127.0.0.1, so the audit line identifies the *destination and the rule*, not the agent. An
    auditable per-agent identity needs the agent to present one (a header the proxy requires, or
    one proxy port per agent) and is not in this layer; it is recorded as a gap rather than faked
    by asserting something the socket cannot tell us.

Fail-closed, in both directions:
  * a policy or a brokered credential that cannot be read kills the process at startup, and
    `paperclip.service` is bound to this unit, so the agent never starts;
  * a Vault token that cannot be minted or renewed only disables the Vault rule — every request to
    the Vault address is refused with a 403 and a log line, instead of being forwarded
    unauthenticated;
  * anything else that goes wrong at request time refuses the request rather than forwarding it.
"""

from __future__ import annotations

import asyncio
import base64
import fnmatch
import http.client
import json
import os
import re
import ssl
import urllib.parse
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple
from weakref import WeakKeyDictionary

from mitmproxy import connection, ctx, http

DENY_BODY = b"paperclip-egress: destination or credential not permitted\n"
# Templates inside the policy:
#   {cred:NAME}          a file in the credentials directory
#   {vault:client-token} the Vault token this addon minted and keeps in memory
#   {env:VAR}            the unit's environment, for an operator who prefers it there
#   {basic:USER:...}     base64 of `USER:...`, which is how GitHub wants a token
_CRED_TOKEN = re.compile(r"\{cred:([A-Za-z0-9._-]+)\}")
_VAULT_TOKEN = re.compile(r"\{vault:([a-z-]+)\}")
_ENV_TOKEN = re.compile(r"\{env:([A-Za-z_][A-Za-z0-9_]*)\}")
_BASIC_TOKEN = re.compile(r"\{basic:([^:{}]*):(.*)\}")
# A periodic token with an explicit max TTL is a hard bound by design, so renewal is never
# optional: re-authenticate (renew-self) at this fraction of the token's period.
DEFAULT_RENEW_FRACTION = 0.5


class ExpansionError(Exception):
    """A policy value that cannot be resolved — the request is refused, never sent half-built."""


class PaperclipEgress:
    def __init__(self) -> None:
        self.rules: List[Dict[str, Any]] = []
        self.credentials: Dict[str, str] = {}
        self.vault: Dict[str, Any] = {}
        self.vault_token: Optional[str] = None
        self.vault_period = 3600
        self.vault_renew_interval = 1800.0
        # Bound in _load(); the default only keeps attribute access total before that.
        self._creds_dir = Path("/etc/paperclip-egress/credentials")
        # state per client connection: what the CONNECT leg asked for, so the request leg can
        # prove it is the same destination. Keyed by the connection object (like mitmproxy's own
        # proxyauth addon does) because a flow's metadata does not survive CONNECT -> request.
        self.targets: "WeakKeyDictionary[connection.Client, Tuple[str, int]]" = WeakKeyDictionary()
        self._renewal_task: Optional["asyncio.Task[None]"] = None
        # Until the policy and every credential it references have been read, nothing is allowed.
        self.ready = False

    # ── lifecycle ──────────────────────────────────────────────────────────
    def load(self, loader) -> None:
        loader.add_option(
            "paperclip_egress_policy",
            str,
            "/etc/paperclip-egress/policy.json",
            "Allowlist/injection policy rendered by Ansible (roles/paperclip-egress).",
        )
        loader.add_option(
            "paperclip_egress_credentials_dir",
            str,
            "/etc/paperclip-egress/credentials",
            "Directory holding the 0600 credential files the policy references.",
        )

    def configure(self, updated) -> None:
        # `configure` runs once at startup, after these options are parsed and before the
        # listener accepts anything. A failure here is meant to be fatal: an addon that cannot
        # read its policy must not stand in front of the agent pretending to be a chokepoint.
        if not self.ready or "paperclip_egress_policy" in updated or "paperclip_egress_credentials_dir" in updated:
            try:
                self._load()
            except Exception as exc:
                # mitmproxy wraps addon callbacks in `safecall`, so raising alone would only be
                # logged — and a chokepoint that answers "denied" to everything looks like a network
                # problem to the agent. Stop the process instead: the unit goes inactive, `BindsTo=`
                # takes the agent down with it, and the journal carries this line and the traceback.
                ctx.log.error(f"paperclip-egress: refusing to run — {exc}")
                shutdown = getattr(ctx.master, "shutdown", None)
                if callable(shutdown):
                    shutdown()
                raise

    def running(self) -> None:
        # The AppRole bootstrap is the startup canary for the brokered Vault path: it runs here,
        # blocking, because a container that cannot log in to Vault should say so once at start
        # rather than behave differently from the first agent run onwards.
        self._vault_bootstrap()
        loop = getattr(ctx.master, "event_loop", None)
        if loop is not None:
            self._renewal_task = loop.create_task(self._vault_renewal_loop())
        ctx.log.info(
            f"paperclip-egress ready: {len(self.rules)} policy rules, "
            f"{len(self.credentials)} credential(s) loaded, "
            f"vault token {'minted' if self.vault_token else 'NOT minted'}"
        )

    def done(self) -> None:
        if self._renewal_task is not None:
            self._renewal_task.cancel()
            self._renewal_task = None

    def _load(self) -> None:
        policy_path = Path(os.path.expanduser(ctx.options.paperclip_egress_policy))
        creds_dir = Path(os.path.expanduser(ctx.options.paperclip_egress_credentials_dir))
        raw = policy_path.read_text(encoding="utf-8")  # a missing policy raises: fail-closed
        policy = json.loads(raw)
        rules = policy.get("rules") or []
        if not rules:
            raise RuntimeError(f"{policy_path}: no rules — refusing to run an empty chokepoint")
        credentials = self._read_credentials(raw, creds_dir)
        vault = policy.get("vault") or {}
        if vault:
            self.vault = vault
            fraction = float(vault.get("renew_fraction") or DEFAULT_RENEW_FRACTION)
            self.vault_renew_interval = max(30.0, self.vault_period * fraction)
        # All-or-nothing assignment: a half-loaded policy is worse than none.
        self.rules = rules
        self.credentials = credentials
        self.ready = True
        self._creds_dir = creds_dir
        ctx.log.info(f"paperclip-egress policy {policy_path} loaded")

    def _read_credentials(self, raw_policy: str, creds_dir: Path) -> Dict[str, str]:
        """Read exactly the credentials the policy references, and insist on all of them.

        Failing at startup — instead of at the first push, hours later — is the whole point: a
        rule whose credential is missing would otherwise forward the agent's request
        unauthenticated, which looks like a target-side permission problem and hides the real
        fault. The Vault wrapping token is deliberately *not* in this set: it is single-use, so
        its absence (or its previous consumption) must disable only the Vault rule, not the whole
        chokepoint.
        """
        wanted = sorted(set(_CRED_TOKEN.findall(raw_policy)))
        credentials: Dict[str, str] = {}
        missing: List[str] = []
        for name in wanted:
            try:
                credentials[name] = self._read_secret_file(creds_dir, name)
            except OSError:
                missing.append(name)
        if missing:
            raise RuntimeError(
                "missing or empty credential file(s) in " + str(creds_dir) + ": " + ", ".join(missing)
            )
        return credentials

    @staticmethod
    def _read_secret_file(creds_dir: Path, name: str) -> str:
        path = creds_dir / name
        value = path.read_text(encoding="utf-8").rstrip("\n")
        if not value:
            raise OSError(f"{path} is empty")
        mode = path.stat().st_mode & 0o777
        if mode & 0o077:
            # Warning, not failure: the layer renders these 0600, so a wider mode means a human
            # changed it — say so loudly in the journal without taking the agent down.
            ctx.log.warn(f"paperclip-egress credential {path} is mode {mode:o}, expected 0600")
        return value

    # ── Vault: unwrap, login, renew, inject ────────────────────────────────
    def _vault_bootstrap(self) -> None:
        """Consume the delivered wrapping token and mint the AppRole client token.

        Called at startup and retried by the renewal loop as long as no token is held. The
        wrapping token and the SecretID inside it are both single-use by design, so this can
        only ever succeed once per provisioning run — which is exactly why the failure is
        reported loudly and why the operator step is "re-run the layer" rather than "restart the
        proxy".
        """
        if not self.vault:
            return
        try:
            wrapped = self._read_secret_file(self._creds_dir, self.vault["wrapping_token_file"])
            # The role_id is a Vault-assigned UUID (the Vault layer exports it; it is an
            # identifier, not a secret), delivered the same way as everything else so this layer
            # does not depend on how the Vault side exports it.
            role_id = self._read_secret_file(self._creds_dir, self.vault["role_id_file"])
            unwrapped = self._vault_call("POST", "/v1/sys/wrapping/unwrap", token=wrapped)
            secret_id = (unwrapped.get("data") or {}).get("secret_id") or (
                unwrapped.get("auth") or {}
            ).get("client_token")
            if not secret_id:
                raise RuntimeError(
                    "unwrap returned neither data.secret_id nor auth.client_token — check what the "
                    "Vault layer wrapped"
                )
            login = self._vault_call(
                "POST",
                self.vault["login_path"],
                body={"role_id": role_id, "secret_id": secret_id},
            )
            auth = login.get("auth") or {}
            token = auth.get("client_token")
            if not token:
                raise RuntimeError("the AppRole login returned no client token")
            # A periodic token reports its period; a non-periodic one only its lease, and either
            # way we renew (never at the last moment) rather than trusting a lifetime.
            self.vault_period = int(auth.get("period") or auth.get("lease_duration") or 3600)
            self.vault_renew_interval = max(
                30.0, self.vault_period * float(self.vault.get("renew_fraction") or DEFAULT_RENEW_FRACTION)
            )
            self.vault_token = token
            ctx.log.info(
                f"paperclip-egress vault: AppRole login ok (period {self.vault_period}s, "
                f"renewing every {int(self.vault_renew_interval)}s); the token stays in memory"
            )
        except Exception as exc:  # noqa: BLE001 — any failure means "no Vault credential"
            self.vault_token = None
            ctx.log.error(
                f"paperclip-egress vault: AppRole bootstrap failed ({exc}); requests to the Vault "
                "address are refused until a new wrapped SecretID is delivered (single-use)"
            )

    def _vault_renew(self) -> None:
        if not self.vault_token:
            self._vault_bootstrap()
            return
        response = self._vault_call(
            "POST",
            "/v1/auth/token/renew-self",
            body={"increment": f"{self.vault_period}s"},
            token=self.vault_token,
        )
        auth = response.get("auth") or {}
        # Keep the current token if the renewal did not echo it: renewal extends a token, it does
        # not replace it.
        token = auth.get("client_token") or self.vault_token
        period = int(auth.get("period") or auth.get("lease_duration") or self.vault_period)
        self.vault_period = period
        self.vault_renew_interval = max(
            30.0, period * float(self.vault.get("renew_fraction") or DEFAULT_RENEW_FRACTION)
        )
        self.vault_token = token
        ctx.log.info(f"paperclip-egress vault: token renewed (next renewal in {int(self.vault_renew_interval)}s)")

    async def _vault_renewal_loop(self) -> None:
        while True:
            await asyncio.sleep(self.vault_renew_interval)
            try:
                # Off the event loop: a hung Vault must not stall every other request through the
                # proxy while we wait for it.
                await asyncio.to_thread(self._vault_renew)
            except Exception as exc:  # noqa: BLE001
                # Fail closed. The explicit max TTL of a periodic token is a hard bound by design,
                # so reaching this state is normal at the end of the window; what must never happen
                # is a request to Vault leaving unauthenticated.
                self.vault_token = None
                ctx.log.error(
                    f"paperclip-egress vault: renewal failed ({exc}); Vault requests are refused "
                    "and a new wrapped SecretID is needed"
                )

    def _vault_call(
        self, method: str, path: str, body: Optional[Dict[str, Any]] = None, token: Optional[str] = None
    ) -> Dict[str, Any]:
        """One HTTPS call to Vault from inside the addon.

        Not through the proxy: `http.client` opens a direct socket, and the chokepoint's own uid is
        allowed out by the packet filter, so the proxy does not have to be able to proxy itself.
        """
        url = urllib.parse.urlsplit(self.vault["address"])
        if url.scheme != "https":
            raise RuntimeError(f"the Vault address must be https, got {self.vault['address']!r}")
        context = ssl.create_default_context()
        if self.vault.get("ca_file"):
            # Only needed when the Vault endpoint is not signed by a public CA — the lab's Vault
            # is reached over Tailscale with a public certificate, so this is empty by default.
            context.load_verify_locations(self.vault["ca_file"])
        connection = http.client.HTTPSConnection(
            url.hostname,
            url.port or 443,
            timeout=float(self.vault.get("timeout") or 10),
            context=context,
        )
        try:
            headers = {"Content-Type": "application/json"}
            if token:
                headers["X-Vault-Token"] = token
            connection.request(method, path, json.dumps(body) if body is not None else None, headers)
            response = connection.getresponse()
            payload = response.read()
            if response.status >= 400:
                # Vault puts its own explanation in the body (errors[], warnings[]) and it is the
                # text an operator needs, so it goes into the log line — and the wrapping token /
                # SecretID never appear in a Vault error response.
                raise RuntimeError(f"{method} {path} -> HTTP {response.status}: {payload[:300]!r}")
            return json.loads(payload or b"{}")
        finally:
            connection.close()

    # ── matching ───────────────────────────────────────────────────────────
    def _match(self, host: str, port: int, path: Optional[str], method: Optional[str]) -> Optional[Dict[str, Any]]:
        """First rule that covers this destination.

        `path`/`method` are None on the CONNECT leg, where neither is known yet; a rule that
        narrows to a path must still *allow* the destination there, and only narrow its injection
        on the request leg. Hence "unknown means do not enforce".
        """
        for rule in self.rules:
            ports = rule.get("ports")
            # Compared as strings: the policy is rendered from Jinja, where a templated port can
            # arrive as "8006" while mitmproxy hands us the integer 8006 — a type mismatch here
            # would silently disable a rule, which is the one failure mode this addon must not have.
            if ports and str(port) not in {str(p) for p in ports}:
                continue
            if not self._host_matches(rule, host):
                continue
            paths = rule.get("paths")
            if paths and path is not None and not any(fnmatch.fnmatchcase(path, p) for p in paths):
                continue
            methods = rule.get("methods")
            if methods and method is not None and method.upper() not in {m.upper() for m in methods}:
                continue
            return rule
        return None

    @staticmethod
    def _host_matches(rule: Dict[str, Any], host: str) -> bool:
        for candidate in rule.get("hosts") or []:
            candidate = candidate.lower()
            if host == candidate:
                return True
            # Subdomain match requires the label boundary, so `github.com` never covers
            # `notgithub.com` — the difference between an allowlist and a suffix grep.
            if rule.get("host_suffix") and host.endswith("." + candidate):
                return True
        return False

    # ── hooks ──────────────────────────────────────────────────────────────
    def http_connect(self, flow: http.HTTPFlow) -> None:
        target = (flow.request.host.lower(), flow.request.port)
        self.targets[flow.client_conn] = target
        if not self.ready:
            self._deny(flow, "connect", target, "policy not loaded")
            return
        rule = self._match(target[0], target[1], None, None)
        if rule is None:
            self._deny(flow, "connect", target, "not on the allowlist")
            return
        ctx.log.info(f"paperclip-egress ALLOW connect {target[0]}:{target[1]} rule={rule['name']}")

    def request(self, flow: http.HTTPFlow) -> None:
        host = flow.request.pretty_host.lower()
        port = flow.request.port
        target = (host, port)
        if not self.ready:
            self._deny(flow, "request", target, "policy not loaded")
            return
        connected = self.targets.get(flow.client_conn)
        if connected is not None and connected[0] != host:
            # Domain fronting: the connection was opened to an allowlisted host, but this request
            # asks for a different one (through a shared CDN address, for instance). The
            # connection's own SNI is only logged, because it is legitimately absent for an
            # IP-address destination and cannot be enforced for a client we do not control.
            self._deny(flow, "request", target, f"host does not match the CONNECT authority {connected[0]}")
            return
        path = flow.request.path.split("?", 1)[0]
        rule = self._match(host, port, path, flow.request.method)
        if rule is None:
            self._deny(flow, "request", target, "not on the allowlist")
            return
        self._inject(flow, rule, target, path)

    def tls_clienthello(self, data) -> None:
        """Log a ClientHello whose SNI disagrees with the authority we were asked for.

        Detection only, and deliberately so: the upstream destination is fixed by the CONNECT
        authority, so an SNI mismatch cannot move the connection — it can only make an allowlisted
        host serve another name, which the Host check in `request` refuses. Kept because "a
        domain-fronting attempt is logged or blocked" is an acceptance criterion, and this is the
        logging half.
        """
        try:
            target = self.targets.get(data.context.client)
            sni = data.client_hello.sni
        except Exception:  # noqa: BLE001 — a logging-only hook must never break a connection
            return
        if target and sni and sni.lower() != target[0]:
            ctx.log.warn(
                f"paperclip-egress SNI-MISMATCH connect={target[0]}:{target[1]} sni={sni} "
                "(the connection still goes to the CONNECT authority)"
            )

    # ── injection ──────────────────────────────────────────────────────────
    def _inject(self, flow: http.HTTPFlow, rule: Dict[str, Any], target: Tuple[str, int], path: str) -> None:
        brokered = bool(rule.get("set_headers") or rule.get("strip_headers"))
        set_headers = rule.get("set_headers") or {}
        try:
            expanded = {name: self._expand(template) for name, template in set_headers.items()}
        except ExpansionError as exc:
            # A credential we cannot produce is a refusal, not an unauthenticated request: this is
            # the fail-closed path for the Vault token, and it is the reason the expansion runs
            # before anything is written to the request.
            self._deny(flow, "inject", target, f"rule {rule['name']}: {exc}")
            return
        for name in rule.get("strip_headers") or []:
            # Drop the client's own value before adding ours: what leaves this container must be
            # the brokered credential and nothing the agent chose to send.
            if name in flow.request.headers:
                del flow.request.headers[name]
        for name, value in expanded.items():
            flow.request.headers[name] = value
        if brokered:
            # The audit line the design asks for: which call, to where, under which rule. No
            # credential value ever reaches the log.
            ctx.log.info(
                f"paperclip-egress INJECT rule={rule['name']} {flow.request.method} "
                f"https://{target[0]}:{target[1]}{path} client={self._peer(flow)}"
            )
        else:
            ctx.log.info(
                f"paperclip-egress ALLOW {flow.request.method} https://{target[0]}:{target[1]}{path} "
                f"rule={rule['name']} client={self._peer(flow)}"
            )

    def _expand(self, template: str) -> str:
        # Order matters: credentials first, then the Vault reference, then the `basic` wrapper
        # around the result.
        expanded = _CRED_TOKEN.sub(lambda m: self.credentials[m.group(1)], template)
        expanded = _VAULT_TOKEN.sub(self._vault_reference, expanded)
        expanded = _ENV_TOKEN.sub(lambda m: os.environ.get(m.group(1), ""), expanded)

        def _basic(match: "re.Match[str]") -> str:
            # The base64 of `user:value` only — the policy writes the `Basic ` scheme itself, so the
            # header reads like the header it is.
            user = match.group(1)
            value = match.group(2)
            return base64.b64encode(f"{user}:{value}".encode("utf-8")).decode("ascii")

        return _BASIC_TOKEN.sub(_basic, expanded)

    def _vault_reference(self, match: "re.Match[str]") -> str:
        key = match.group(1)
        if key != "client-token":
            raise ExpansionError(f"unknown vault reference '{key}'")
        if not self.vault_token:
            raise ExpansionError(
                "no Vault token (the AppRole bootstrap did not succeed; see the earlier log line)"
            )
        return self.vault_token

    # ── refusals ───────────────────────────────────────────────────────────
    def _deny(self, flow: http.HTTPFlow, kind: str, target: Tuple[str, int], detail: str) -> None:
        ctx.log.warn(
            f"paperclip-egress DENY {kind} {target[0]}:{target[1]} detail={detail} client={self._peer(flow)}"
        )
        flow.response = http.Response.make(
            403,
            DENY_BODY,
            {"content-type": "text/plain; charset=utf-8", "x-paperclip-egress": "denied"},
        )

    @staticmethod
    def _peer(flow: http.HTTPFlow) -> str:
        peer = getattr(flow.client_conn, "peername", None)
        if isinstance(peer, tuple) and peer:
            return str(peer[0])
        return "unknown"


addons = [PaperclipEgress()]

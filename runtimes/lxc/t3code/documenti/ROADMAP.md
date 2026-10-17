# Roadmap — T3 Code in laboratorio (CT106)

Le tappe, in ordine, con il criterio per dichiararle chiuse. La tappa **R0** e' fatta (questo passo);
le altre sono proposte all'operatore e diventano task della coda solo se le accetta.

## R0 — Ricerca e proposta *(fatto, 2026-10-17)*

- Ricerca verificata sulla sorgente upstream (`pingdotgg/t3code`, `main`) e sulla documentazione;
  release stabile `0.1.0` installata in prova e interrogata.
- Prodotto: `runtimes/lxc/t3code/PROPOSTA.md` + il layer eseguibile.
- **Chiuso quando**: la proposta risponde alle sei domande e i file del layer passano i controlli
  statici (`bash -n`, `t3.json` validato contro `https://t3.codes/schema/t3.json`).

## R1 — Installazione in CT106 *(proposta)*

- Eseguire `verify-t3code.sh` (preflight), poi `install-t3code.sh` con root, poi di nuovo la verifica.
- Comprende le due azioni che nessuno script puo' fare al posto dell'operatore: `tailscale up` sul
  nodo (oggi `Logged out`) e i login dei provider che si vogliono accendere.
- **Chiuso quando**: `verify-t3code.sh` esce 0 e `systemctl --user status t3code.service` e' `active`
  dopo un `reboot` del container, con `Linger=yes`.

## R2 — La prova: laptop, telefono, un repo vero *(proposta)*

- `t3 pair --tailscale`, app desktop in modalita' controller, app mobile accoppiata.
- Clone **dedicato** di un repository reale, `t3.json` e hook del worktree committati, primo thread in
  worktree nuovo su OpenCode con permission mode `Supervised`.
- **Chiuso quando**: il task e' stato revisionato e approvato dal telefono, il worktree e' stato
  creato con il setup eseguito, e il cap di memoria ha tenuto (`MemoryCurrent < MemoryMax`).

## R3 — Decisione sulla convivenza *(proposta, dipende da R2)*

- Con la prova in mano: T3 Code resta su repository separato, o si porta sul repository di Ambrogio?
- Se si porta: politiche di cleanup (`Any local files`), auto-pull spento, scope di pairing senza
  `source-control:write`, e la regola che la pubblicazione resta di `Release`.
- **Chiuso quando**: la decisione e' scritta nel progetto con la ragione, e — se e' "stesso
  repository" — le tre mitigazioni sono applicate e verificate.

## R4 — Irrigidimento, solo se serve *(condizionale)*

- Utente di servizio dedicato (`t3code`) con i suoi login, cosi' i token dei provider non stanno
  nella home di Ambrogio.
- Valutazione di T3 Connect per le notifiche push, con il prezzo opposto dichiarato.
- Integrazione in Ansible (`layer4-ai.yml`) se la prova diventa esercizio quotidiano.
- **Chiuso quando**: o si fa, o l'operatore dice che non serve — in entrambi i casi resta scritto.

## Fuori dalla roadmap

- Sostituire Ambrogio con T3 Code: non e' un orchestratore con coda durevole, stadi, memoria di
  progetto e cancello di rilascio (§4 della proposta).
- Esporre T3 Code a Internet: la rotta della prova e' la tailnet.

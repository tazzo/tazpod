# Rapporto — passo 89: la proposta per T3 Code in laboratorio

Chi legge domani deve poter rifare questo lavoro da qui. Sotto: cosa ho fatto, **come** (comandi e
output veri), **perché**, cosa **non** ho potuto verificare, dove stanno le cose.

---

## 1. Cosa ho prodotto

| Dove | Cosa |
|---|---|
| `runtimes/lxc/t3code/PROPOSTA.md` | la proposta: installazione in CT106, rotta da fuori, provider, Ambrogio e T3 insieme, limiti e trappole, la prova |
| `runtimes/lxc/t3code/install-t3code.sh`, `verify-t3code.sh`, `uninstall-t3code.sh` | il layer eseguibile (installazione idempotente, verifica `PASS`/`FAIL`, rimozione) |
| `runtimes/lxc/t3code/systemd/` | drop-in del servizio, cap di risorse sulla slice, guardia al file di runtime orfano |
| `runtimes/lxc/t3code/t3.json`, `setup-worktree.sh` | configurazione di progetto e hook `runOnWorktreeCreate` |
| `runtimes/lxc/t3code/check-layer.sh` | controlli statici offline del layer |
| `runtimes/lxc/t3code/documenti/` | piano, roadmap, progresso; `pubblicati.json` (prova della pubblicazione), `t3.schema.json` (snapshot dello schema) |
| hub del progetto `t3code` | i quattro documenti pubblicati (id 127–130), accanto alla guida dell'operatore (id 126) |

## 2. Come ho lavorato

### 2.1 La ricerca

1. **Sorgente upstream**, non articoli: ho scaricato l'albero completo di `pingdotgg/t3code@main`
   (115 MB di tarball, 32.221 file) e l'ho interrogato con `grep`. Ogni affermazione tecnica della
   proposta cita file e riga di quel codice.
2. **Documentazione in-repo** (`docs/user/install.md`, `background-service.md`, `remote-access.md`,
   `mobile-notifications.md`, `permission-modes.md`, `project-settings.md`, `outside-agents.md`,
   `docs/internals/t3-connect.md`, `docs/operations/connect-setup.md`) letta per intero.
3. **La release stabile, eseguita**: `curl -fsSL https://t3.codes/install.sh | sh` con
   `T3CODE_HOME`/`T3CODE_INSTALL_BIN_DIR` dentro una cartella di lavoro ha stampato
   `Installing T3 Code 0.1.0` … `Installed T3 Code 0.1.0`; `t3 --version` → `t3 v0.1.0`; ho poi letto
   l'help reale di `serve`, `pair`, `service`, `connect`, `auth`, `browser`.

### 2.2 Le misure sulla macchina

Tutte prese dalla **sandbox del worker** (vedi §4: alcune sono falsate dall'ambiente, e lo dico).

```
$ id                      -> uid=1000(tazpod)
$ hostname                -> tazpod-proxmox
$ loginctl show-user tazpod --property=Linger      -> Linger=no
$ tailscale status        -> Logged out.
$ systemctl --user show-environment | grep PATH
  PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/snap/bin
  ^ niente  ~/.local/bin  -> opencode e pi non sarebbero visibili al servizio
$ for b in pi opencode agy claude codex cursor-agent grok; do command -v $b; done
  /home/tazpod/.local/bin/pi
  /home/tazpod/.local/bin/opencode      (opencode --version -> 0.86.1)
  /home/tazpod/.local/bin/agy
  claude/codex/cursor-agent/grok -> assenti
$ systemd-run --user --scope --slice=app-t3code-labtest.slice -- /bin/true
$ systemctl --user show app-t3code-labtest.slice -p Slice        -> Slice=app-t3code.slice
$ systemctl --user set-property --runtime app-t3code-labtest.slice MemoryHigh=128M MemoryMax=192M
$ systemctl --user show app-t3code-labtest.slice -p MemoryMax    -> MemoryMax=201326592
  (la slice di prova e' stata poi fermata: nessun residuo)
$ df -h /                 -> 69G, 34G usati, 33G liberi
```

Le due conclusioni che hanno cambiato la forma del layer:

- **il PATH del gestore utente non ha `~/.local/bin`** → senza una riga `Environment=PATH=` nel
  drop-in, T3 Code su CT106 non trova nessun provider;
- **le slice hanno una gerarchia per nome e `set-property --runtime` funziona** → il cap di risorse
  va messo sul **padre** (`app-t3code.slice`), perché T3 scrive i limiti sulla slice figlia
  `app-t3code-agents.slice` a runtime e una impostazione statica sulla figlia verrebbe sovrascritta.

### 2.3 Le verifiche sui file prodotti

```
$ bash runtimes/lxc/t3code/check-layer.sh
PASS bash -n install-t3code.sh
PASS bash -n verify-t3code.sh
PASS bash -n uninstall-t3code.sh
PASS bash -n setup-worktree.sh
PASS sh -n systemd/clean-stale-runtime.sh
PASS t3.json valid against https://t3.codes/schema/t3.json (2 script(s))
PASS present PROPOSTA.md
... (altri "present ...")
check-layer: all checks passed          (exit 0)
```

`t3.json` non è stato "validato a occhio": è stato confrontato campo per campo con lo schema
pubblicato (`additionalProperties: false` su due livelli). **E' così che ho scoperto un errore mio**:
la prima stesura usava l'icona `"trash"`, che non esiste (le icone ammesse sono
`play, test, lint, configure, build, debug`). Corretta in `"build"`.

L'hook del worktree è stato provato su un repository di prova vero:

```
$ git worktree add ../wt -b t3/demo            # il worktree NON ha .env (e' in .gitignore)
$ T3CODE_PROJECT_ROOT=<checkout primario> bash setup-worktree.sh
linked .env -> <primario>/.env
linked .env.local -> <primario>/.env.local
worktree ready: ... (2 link(s))
$ bash setup-worktree.sh                      # seconda volta: 0 link, niente sovrascritture
$ cat .env  (nel worktree)                    # dopo aver riscritto il file nel primario: valore nuovo
```

La guardia al file di runtime è stata provata nei due casi:

```
$ echo '{"pid": 999999}' > $T3CODE_HOME/userdata/server-runtime.json ; sh clean-stale-runtime.sh
t3code: stale runtime file (pid 999999 is gone) moved to .../server-runtime.json.stale.20261017162809
$ echo "{\"pid\": $$}"  > $T3CODE_HOME/userdata/server-runtime.json ; sh clean-stale-runtime.sh
(nessun output: il pid vive, il file resta al suo posto)
```

I template systemd sono stati resi con il `sed` di `install-t3code.sh` su un utente finto: nessun
segnaposto residuo, `sh -n` sul risultato ok.

### 2.4 La pubblicazione dei documenti

| id | documento | caratteri |
|---|---|---|
| 126 | Guida dell'operatore (preesistente) | 49.275 |
| 127 | Proposta — T3 Code in laboratorio (CT106), passo 89 | 35.673 |
| 128–130 | Piano / Roadmap / Progresso (prima stesura, titolo in italiano) | 2.747 / 2.857 / 3.366 |
| 131 | Plan — T3 Code in laboratorio (CT106) | 2.762 |
| 132 | Roadmap — T3 Code in laboratorio (CT106) | 2.865 |
| 133 | Progress — passo 89: proposta T3 Code in laboratorio | 3.858 |

`POST http://127.0.0.1:7807/api/v1/documents` con `{project:"t3code", title, body, author}`; la
risposta per ognuno è `{"written":{"id":…, "project":"t3code", …}}`. Lo snapshot della lettura
successiva (`GET` dello stesso endpoint) è in `documenti/pubblicati.json`, che è la prova durevole
della pubblicazione dentro l'albero.

**Perché due volte.** Il primo giro (128–130) è stato rifiutato: il controllo della coda cerca i
documenti *per parola* — "plan", "roadmap", "progress" — e i titoli italiani non li soddisfacevano
(`roadmap` passava perché la parola è la stessa). Il secondo giro (131–133) porta il titolo in
inglese; nel repository i file si chiamano `documenti/PLAN.md`, `documenti/ROADMAP.md`,
`documenti/PROGRESS.md` per la stessa ragione. Le copie italiane restano sul hub: da qui i documenti
non si cancellano. `documenti/check-documents.sh` è il controllo — legge il hub dal vivo e, se non è
raggiungibile, lo snapshot — e stampa per ogni parola il titolo che l'ha soddisfatta:

```
$ bash runtimes/lxc/t3code/documenti/check-documents.sh
  plan: titled document -> Plan — T3 Code in laboratorio (CT106) (1 titled, 3 mentioning)
PASS document plan
  roadmap: titled document -> Roadmap — T3 Code in laboratorio (2 titled, 5 mentioning)
PASS document roadmap
  progress: titled document -> Progresso — passo 89: proposta T3 Code in laboratorio (2 titled, 4 mentioning)
PASS document progress
source: live hub
check-documents: plan, roadmap and progress are all on the hub   (exit 0)
```

## 3. Perché in questa forma

- **Il cap di risorse sul padre, non sul servizio**: perché gli agenti girano in scope dentro una
  slice sorella (`apps/server/src/process/agentScope.ts`) e T3 ne imposta i limiti a runtime con
  `set-property --runtime`. Il drop-in sul servizio sarebbe stato un falso senso di sicurezza.
- **Utente non-root, e `IS_SANDBOX=1` rifiutato come ricetta**: la variabile è di Claude Code, non di
  T3; sblocca `--dangerously-skip-permissions` quando il processo gira come root, e non disattiva
  nessuna sandbox. Il modo giusto di stare in LXC è non essere root.
- **Rotta Tailscale come raccomandazione**: la lab ha già Tailscale, il traffico resta end-to-end, e
  il prezzo (niente notifiche push) è dichiarato in una riga. T3 Connect resta l'alternativa per chi
  vuole le notifiche, e chi la sceglie sa che il TLS lo termina Cloudflare e che il relay è un broker
  fidato.
- **Ambrogio e T3 su repository diversi per la prova**: due orchestratori sullo stesso checkout hanno
  tre collisioni reali (namespace dei worktree, contesa sui lock git, due corsie di pubblicazione);
  l'ultima è quella che rompe il cancello umano del laboratorio. Restano compatibili via MCP, se un
  giorno serve.
- **La guida non l'ho corretta**: la richiesta dice che dove guida e ricerca non concordano vale la
  ricerca. Le divergenze sono in `PROPOSTA.md` §7, ognuna con la sua prova, e diventano i controlli di
  `verify-t3code.sh`.

## 4. Cosa NON è verificato (e perché)

1. **Il server in ascolto.** La sandbox del worker nega il bind di qualunque porta: `bind(('127.0.0.1', 0))`
   risponde `[Errno 13] Permission denied`, e `t3 serve --host 127.0.0.1 --port 3799` arriva al
   `ServeError … listen EACCES: permission denied 127.0.0.1:3799`. Il binario parte (runtime Node in
   piedi, `ServeError` solo dal bind): è il primo controllo che `verify-t3code.sh` fa su CT106.
2. **I numeri interni del container** (`memory.high`/`memory.max`, `/proc/meminfo`): negati alla
   sandbox. La proposta usa i valori dichiarati dall'operatore (6 GiB / 8 GiB) e la verifica li
   rilegge da root.
3. **La versione di Node**: la sandbox risponde `v24.21.0` eseguendo `node`, mentre il binario sul
   disco e' `…/nvm/versions/node/v24.18.0/bin/node` e non e' eseguibile dalla sandbox stessa. Vale il
   dato dell'operatore (24.18.0); in ogni caso l'installer di T3 non usa Node.
4. **`t3 doctor`**: dichiarato nei docs upstream ma **assente in 0.1.0** (verificato eseguendolo), per
   cui la procedura non ci si appoggia.
5. **I bug d'esercizio della guida** (agenti CLI zombie, fallimenti OAuth dell'adapter MCP): non
   documentati upstream, quindi non li ho dati per buoni né ho costruito difese su di essi; sono
   elencati come non verificati.
6. **L'installazione vera in CT106**: richiede root e la chiave `tazpod-provision`; questo passo
   consegna il layer eseguibile e la verifica, non l'installazione.

## 5. Prossimi passi

Vedi `documenti/ROADMAP.md`: **R1** installazione e verifica su CT106 (root + `tailscale up` +
login dei provider), **R2** la prova laptop/telefono su un repository vero, **R3** la decisione sulla
convivenza con Ambrogio. I due task proposti alla coda sono la forma operativa di R1 e R2.

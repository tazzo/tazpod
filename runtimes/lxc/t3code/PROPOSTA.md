# Proposta — T3 Code in laboratorio (CT106)

**Stato**: proposta, pronta da eseguire. **Data**: 2026-10-17. **Autore**: worker della coda Ambrogio
(passo 89), su richiesta dell'operatore del 2026-10-17.

Questa proposta risponde alle sei domande poste con la guida dell'operatore
(`Guida — Installazione T3 Code in laboratorio`, documento del progetto). Dove la guida e la
ricerca non dicono la stessa cosa **vale la ricerca**, e §7 dice punto per punto cosa non torna.

I file che rendono eseguibile questa proposta stanno in questa stessa cartella
(`runtimes/lxc/t3code/`): §10 li elenca e dice come si lanciano.

---

## 0. Metodo, e cosa vuol dire "verificato" qui

La ricerca è stata fatta il **2026-10-17** su tre fonti primarie:

1. la **sorgente upstream** (`pingdotgg/t3code`, branch `main`): albero Git completo scaricato e
   interrogato (32.221 file). Ogni affermazione di architettura qui sotto cita file e riga di quel
   codice, non un articolo;
2. la **documentazione ufficiale** nel repository (`docs/user/*.md`, `docs/operations/*.md`,
   `docs/internals/*.md`);
3. la **release stabile**, installata per davvero in una cartella di prova su questa macchina:
   `t3 v0.1.0`, installer ufficiale, `t3 --help`, `t3 serve --help`, `t3 pair --help`,
   `t3 service --help`, `t3 connect --help` eseguiti e letti.

**Un avvertimento che vale per tutto il documento.** Le "misure sulla macchina" fatte da questa
sessione passano per la *sandbox del worker* Ambrogio, che virtualizza parte del filesystem e
**vieta il bind di qualunque porta** (verificato: anche `127.0.0.1:0` risponde `EACCES`). Quindi:

- i numeri che l'operatore ha già misurato (indirizzo `192.168.1.206`, Node `v24.18.0`,
  `claude`/`codex` assenti, `Linger=no`, 6 GiB / 8 GiB di memoria) **sono la verità di partenza** e
  li uso come tali;
- le misure che ho preso io e che *contraddicono* la sandbox (per esempio `node --version` risponde
  `v24.21.0` mentre il binario sul disco e' `.../nvm/versions/node/v24.18.0/bin/node`, che la
  sandbox non lascia nemmeno eseguire) le segnalo come **provenienti dalla sandbox**, non come
  correzioni;
- le misure che *aggiungono* qualcosa e che nessuno aveva fatto (`Linger=no` riconfermato, PATH del
  gestore utente systemd, gerarchia delle slice, `set-property --runtime` sulle slice, `t3 doctor`
  assente in 0.1.0, `tailscale status` = `Logged out`) sono etichettate come tali.

Per questo il layer spedisce **`verify-t3code.sh`**: è il controllo autorevole, da eseguire su CT106
con root, e dice `PASS`/`FAIL` su ognuna delle cose che possono falsare questa proposta.

---

## 1. Dove e come si installa in CT106

### 1.1 Il metodo

Installer ufficiale, come **utente di servizio**, non root:

```bash
T3CODE_HOME=/home/tazpod/.t3 \
T3CODE_INSTALL_BIN_DIR=/home/tazpod/.local/bin \
  sh -c 'curl -fsSL https://t3.codes/install.sh | sh'      # -> /home/tazpod/.local/bin/t3
```

Verificato eseguendolo qui: scarica l'archivio della release, verifica lo sha256, scompatta in
`~/.t3/runtime/versions/<versione>` e mette il symlink `t3` in `~/.local/bin`. In prova ha stampato
`Installing T3 Code 0.1.0` / `Installed T3 Code 0.1.0`, e `t3 --version` risponde `t3 v0.1.0`.

Due precisazioni che l'operatore deve avere chiare:

- **Node non serve.** L'installer dichiara in testa `Needs only sh, tar, sha256sum or shasum, and curl
  or wget; no Node, npm, or compiler` e nel codice non invoca mai `npm`. Node serve solo per la via
  `npx t3@latest` e per alcuni provider (Pi vuole Node >= 22.19). Su CT106 c'è comunque Node
  `v24.18.0`, quindi il problema non si pone.
- **La release stabile è `0.1.0` e la documentazione upstream è più avanti di lei.** In 0.1.0 il
  comando `t3 doctor` **non esiste** (verificato: `t3 doctor --dry-run` risponde
  `Unrecognized flag: --dry-run in command t3` e stampa l'help di primo livello). La manutenzione del
  database in questa versione è automatica. Vedi §5.

### 1.2 L'utente, e perché non root

**Prova**: utente `tazpod` (quello che esiste già). Ha già `opencode`, `pi`, `agy`, i loro login e
l'accesso ai repository del laboratorio; creare un utente nuovo significherebbe rifare tutti i login
dei provider prima di poter accendere un agente.

**Stato finale consigliato**: utente dedicato `t3code` (senza login interattivo), così i token dei
provider e il database `statev2` di T3 non stanno nella stessa home di Ambrogio. Costo: i
login dei provider vanno rifatti una volta per quell'utente. `install-t3code.sh` prende `T3_USER`,
quindi il passaggio è un parametro, non una riscrittura.

**Mai root**, e qui sta il verdetto su `IS_SANDBOX=1` (§7): lo script crea sempre i file *come*
l'utente di servizio e chiama `loginctl` da root solo per il lingering.

### 1.3 Il servizio, e il `Linger`

```bash
# dentro CT106, come tazpod (install-t3code.sh lo fa per te)
t3 service install      # unit systemd UTENTE:  ~/.config/systemd/user/t3code.service
t3 service status       # stato + percorso del log + verifica del linger
```

Il linger è **obbligatorio** e su questa macchina **manca**:

```
$ loginctl show-user tazpod --property=Linger
Linger=no
```

con `Linger=no` un servizio *utente* muore con l'ultima sessione (e al boot non parte): `t3 service
status` lo segnala come `linger-disabled`, e il rimedio è esattamente quello che scrive la guida:

```bash
sudo loginctl enable-linger tazpod
```

`install-t3code.sh` lo fa prima di installare il servizio, e `verify-t3code.sh` lo controlla dopo.
Lo stesso vale per il gestore utente: senza lingering systemd-logind non avvia `user@1000.service`
al boot, e senza quel gestore T3 Code **non può usare gli scope degli agenti** (lo dice lui stesso:
`Agent scopes are off: no systemd user manager`, `apps/server/src/process/agentScope.ts`).

### 1.4 L'isolamento, e cosa aggiunge questo layer

T3 Code non aggiunge una sandbox propria: fa girare le CLI dei provider come processi figli
dell'utente che lo esegue. La barriera è quindi **il container LXC + l'utente**, e la proposta
aggiunge tre cose concrete:

1. **un cap di risorse** sulla slice che contiene sia il server sia gli agenti
   (`systemd/app-t3code.slice`) — §5 spiega perché non basta il drop-in sul servizio;
2. **un PATH esplicito** per il servizio (`50-lab.conf`), perché l'unit generata da T3 **non imposta
   nessun PATH** e il gestore utente non ha `~/.local/bin` dove stanno `opencode` e `pi` (verificato:
   `systemctl --user show-environment` → `PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:...`,
   niente `~/.local/bin`); senza quella riga T3 Code non trova **nessun** provider su CT106;
3. **una guardia al file di runtime** (`clean-stale-runtime.sh`), che è la trappola "container
   riavviato male" della guida (§7).

### 1.5 I comandi, per esteso

```bash
# dalla macchina di orchestrazione (root, chiave tazpod-provision)
scp -r runtimes/lxc/t3code root@192.168.1.206:/root/
ssh root@192.168.1.206 'T3_USER=tazpod bash /root/t3code/verify-t3code.sh'   # preflight
ssh root@192.168.1.206 'T3_USER=tazpod bash /root/t3code/install-t3code.sh'  # installa
ssh root@192.168.1.206 'T3_USER=tazpod bash /root/t3code/verify-t3code.sh'   # verifica
```

`install-t3code.sh` è idempotente (si rilancia dopo ogni modifica ai file di questo layer) e si
presta ad essere chiamato da Ansible: è uno script unico, senza dipendenze oltre a
`sh`/`curl`/`tar`/`sha256sum`, ed è il candidato naturale per un `layer4-ai.yml` (accanto a pi,
gemini, opencode, omp) quando il passaggio non sarà più una prova.

---

## 2. Come lo si comanda da fuori

### 2.1 La rotta consigliata: **Tailscale**

Il laboratorio ha già Tailscale, e `t3` ha il supporto nativo — `t3 serve --tailscale-serve`,
`t3 pair --tailscale` (verificato: `--tailscale, Publish the server over Tailscale Serve HTTPS and
pair through the tailnet URL`, `--tailscale-serve-port`). La rotta consigliata è quella:

```bash
# su CT106, come tazpod, una volta sola (mapping persistente)
T3_PORT=3773
t3 pair --tailscale                 # pubblica il server su https://<nome-nodo>.<tailnet>.ts.net e stampa il link di pairing
t3 pair                             # oppure, per la sola LAN: link di pairing su 192.168.1.206
```

Il servizio resta in ascolto su loopback; **Tailscale Serve** fa da ponte sulla tailnet con HTTPS
(il certificato lo emette Tailscale), quindi non serve aprire porte né esporre nulla su Internet.

**Il prezzo della scelta, in una riga: con Tailscale il traffico è cifrato end-to-end e non passa da
nessun terzo, ma il telefono non riceve notifiche push — l'app va aperta a mano per sapere che un
agente ha finito** (docs: *"Background delivery requires T3 Connect; a direct or Tailscale connection
alone does not enable push notifications"*).

### 2.2 L'alternativa, e cosa costa

**T3 Connect** (Cloudflare Tunnel + relay Ping Labs) è l'unica strada per le notifiche push su
Android, e attraversa NAT e firewall senza configurazione. In cambio: il TLS lo termina l'edge
Cloudflare e il relay è un **broker fidato** che firma le credenziali di bootstrap verso
l'ambiente (`docs/internals/t3-connect.md`, §"The relay is a trusted broker": *"The relay holds the
signing authority for mint requests"*) — non è end-to-end. Per il laboratorio: **non accenderlo**,
salvo volere espressamente le notifiche.

### 2.3 Lo stato della rete su CT106, misurato

```
$ tailscale status
Logged out.
Log in at: https://login.tailscale.com/a/174115dc2d345d
```

e in `/etc/hosts` ci sono già i nomi della tailnet (`*.magellanic-gondola.ts.net`). Quindi: **prima
di tutto, il nodo va rimesso nella tailnet** (`sudo tailscale up --ssh --hostname=ct106`), poi T3.
La misura viene dalla sandbox del worker: `verify-t3code.sh` la rifà da root e la classifica come
`FAIL tailscale` finché non è a posto.

### 2.4 Laptop e telefono

- **Desktop**: app Electron da GitHub Releases (`sudo apt install ./T3-Code-*.deb`) e in
  `Settings → Connections` spegnere **Local environment** se deve solo comandare CT106 (diventa un
  controller puro, senza agenti locali).
- **Telefono**: app T3 Code da App Store / Play Store (`com.t3tools.t3code`), poi `Add environment`
  con il link di pairing (o scansione del QR).
- **Browser**: `https://<nodo>.<tailnet>.ts.net/` raggiunge la stessa web app. L'app ospitata
  `app.t3.codes` **non** funziona su HTTP semplice: per la LAN va bene il link di pairing, non
  l'app ospitata.

---

## 3. Gli agenti: quali accendere, quali installare

T3 Code non rivende modelli: `Settings → Providers` accende la CLI che **esiste già su CT106** e ne
usa il login. Ricerca sul campo (cosa risponde il PATH dell'utente `tazpod`):

| Provider | Stato su CT106 | Cosa fare |
|---|---|---|
| **OpenCode** | presente — `/home/tazpod/.local/bin/opencode` (`opencode --version` → `0.86.1`) | accendere subito in `Settings → Providers` |
| **Pi** | presente — `/home/tazpod/.local/bin/pi` | accendere subito (vuole Node >= 22.19: c'è) |
| **Antigravity** | CLI presente — `/home/tazpod/.local/bin/agy` (installata da `layer4-ai.yml`) | `Settings → Providers` → *Install Antigravity* / *Sign in with Google* |
| **Claude Code** | assente | `npm i -g @anthropic-ai/claude-code` poi `claude auth login` |
| **Codex** | assente | CLI da `developers.openai.com/codex/cli` poi `codex login` (o "Connect with ChatGPT") |
| **Cursor** | assente | CLI da `cursor.com/cli` poi `agent login` (l'eseguibile si chiama `cursor-agent`) |
| **Grok Build** | assente | CLI da `x.ai/cli` poi `grok login` |
| **Muse Code** | assente | da `dev.meta.ai/docs/muse-code`, `muse login` |

Note che contano:

- **multi-account Claude** senza compromessi: una directory di config per account e un'istanza per
  directory —
  ```bash
  mkdir -p ~/.claude_personal && CLAUDE_CONFIG_DIR=~/.claude_personal claude auth login
  ```
  e in T3: istanza "Claude Work" con `CLAUDE_CONFIG_DIR` vuoto, istanza "Claude Personal" con
  `~/.claude_personal`. La cronologia locale resta separata.
- **Codex con backend alternativi** (Azure): le variabili (`AZURE_API_KEY`, base URL) si mettono
  **nell'istanza del provider**, non in `~/.bashrc` — il servizio non legge nessun rc.
- **Tutte le CLI dei provider devono stare nel PATH del servizio**: vedi il PATH in §1.4. E' il
  motivo per cui `verify-t3code.sh` prova `command -v` *con il PATH dell'unit*, non con il proprio.

---

## 4. Ambrogio e T3 Code insieme (la domanda che conta)

Sono due orchestratori con lo stesso modello di isolamento (un git worktree per task, `.git`
condiviso): la tentazione è fonderli. Non vanno fusi, e la ragione è che **non fanno la stessa
cosa**.

### 4.1 Cosa fa l'uno che l'altro non fa

| | **Ambrogio** (Multica + coda) | **T3 Code** |
|---|---|---|
| Unita di lavoro | task/issue durevole, con stadi e handoff | thread interattivo |
| Chi decide il prossimo passo | la coda e l'operatore (gate esplicito) | chi ha il telefono in mano, adesso |
| Isolamento | worktree per passo (`wt/q<N>`) | worktree per thread (default `~/.t3/worktrees`) |
| Superficie | una conversazione con l'operatore, il hub, i documenti di progetto | web + desktop Electron + **mobile**, terminale e diff dal vivo |
| Approvazioni | cancello dell'operatore prima di pubblicare | permission mode *dentro* il turno (Supervised/Auto/Full), prompt sul telefono |
| Registro | piano/roadmap/progress, memoria, golden rules | SQLite locale (`statev2.sqlite`), nessun RBAC, nessun audit esportabile |
| Multi-provider | gli agenti sono i specialisti del laboratorio (rivestiti da skill e dominio) | qualunque CLI installata: Codex, Claude, Cursor, Grok, OpenCode, Antigravity, Pi, Muse |
| Cosa non ha | nessun terminale live, nessuna app mobile, nessuna approvazione in-stream | nessuna coda durevole, nessuno stadio, nessun gate di rilascio, nessuna memoria di progetto, un solo utente senza ruoli |
| Come parla con l'esterno | issue, commenti, hub, git | **server MCP** (`/mcp`): thread, progetti, provider/model disponibili |

In una riga: **T3 Code è la superficie (occhi e mani, in tempo reale, dal telefono); Ambrogio è il
registro (coda durevole, stadi, memoria, cancello di rilascio).** Il primo non sa *cosa* va fatto
domani; il secondo non sa mostrare un agente che sta lavorando adesso.

### 4.2 Come convivono sullo stesso repository — e perché è meglio non farlo

Tecnicamente due orchestratori possono lavorare sullo stesso repository, perché `git worktree` è
progettato per questo (worktree distinti, un solo `.git`). Le collisioni reali sono tre, e nessuna è
teorica:

1. **Namespace dei branch e delle directory.** T3 Code nomina i branch con prefisso statico `t3/`
   (`Settings → Source Control → Worktree branch naming`, default `t3/`) e mette i worktree in
   `~/.t3/worktrees`; la coda Ambrogio usa `wt/q<N>` in `scratch/lab/wt/`. **Nessuna collisione di
   percorso**, purché si lascino i default (non puntare la *Worktree location* di T3 dentro
   `scratch/lab/wt`).
2. **Operazioni git concorrenti.** `git fetch`/`index.lock` sullo stesso `.git` mentre entrambi
   lavorano: T3 Code ha una logica di retry dedicata ai lock
   (`apps/server/src/vcs/VcsProcess.ts`, "unable to create ... .lock" e' classificato come
   *retryable*), ma è contesa, non coordinazione. Mitigazione: **non** accendere `Automatically pull`
   di T3 sul repository che sta lavorando Ambrogio.
3. **Due corsie di pubblicazione.** Il flusso più comodo di T3 è il bottone sul telefono che fa
   commit + push + PR da solo (`One-Button Pull Request Workflow`). Nel laboratorio la pubblicazione
   è di **Release**, con il cancello dell'operatore: due corsie che spingono sullo stesso repository
   vanificano il gate. Se i due convivono, T3 va accoppiato **senza** lo scope
   `source-control:write` (vedi `t3 pair --scope ...`) e il bottone PR non si usa.

**Verdetto: repository diversi per la prova.** T3 Code lavora su un **clone dedicato** del
repository (per esempio `/home/tazpod/t3lab/tazpod`), con i suoi worktree sotto `~/.t3/worktrees`;
Ambrogio continua sui suoi. Vantaggi: `.git` separati (niente contesa di lock, niente
`git worktree prune` incrociato), politiche di cleanup di T3 che non toccano nulla di Ambrogio,
accessi e scope più stretti, e un incidente di T3 non può danneggiare il repository che l'operatore
usa per la coda. Quando la convivenza sarà routine, l'unica cosa da aggiungere è la **politica di
cleanup** di T3 (§5.3) e il divieto di auto-pull.

### 4.3 Il ponte, se si vuole: T3 come tool di Ambrogio

T3 Code espone un **server MCP** sulla stessa porta (`https://<indirizzo-ambiente>/mcp`, in locale
`http://127.0.0.1:3773/mcp`): un agente esterno può leggere progetti e thread, **avviare e guidare**
thread, e vedere provider e modelli disponibili. Questo permette ad Ambrogio (o a uno specialista)
di trattare T3 come una superficie da pilotare:

```bash
claude mcp add --transport http t3 http://127.0.0.1:3773/mcp
claude mcp login t3        # su una macchina senza browser: claude mcp login --no-browser
```

Non è la strada consigliata per la prima prova — è un secondo canale di orchestrazione sopra il
primo — ma è la risposta onesta a "come si parlano": **via MCP**, con permessi scelti al momento
dell'approvazione (read only, supervised, full access).

---

## 5. Limiti di risorse, disco, e le trappole che valgono davvero

### 5.1 I limiti di risorsa, e perché vanno sulla slice e non sul servizio

I numeri di partenza (dati dall'operatore, e coerenti con la storia del laboratorio): CT106 ha
`lxc.cgroup2.memory.high = 6 GiB` e `memory.max = 8 GiB`, e sullo stesso container girano già il
motore Ambrogio, `opencode`, `pi` e `omp`. L'incidente del 2026-10-02 (`memory/reports/`) e' la
prova che "agenti + browser headless + Node" saturano questa macchina.

Cosa fa T3 Code da solo, letto nel codice (`apps/server/src/process/agentScope.ts`):

```ts
high: Math.max(totalBytes - 6*GiB, Math.floor(totalBytes / 2)),
max:  Math.max(totalBytes - 4*GiB, Math.floor(totalBytes * 0.75)),
```

applicati con `systemctl --user set-property --runtime app-t3code-agents.slice MemoryHigh=… MemoryMax=…`.
Su CT106 questo significa **4 GiB di high e 6 GiB di max** (con `totalBytes` = 8 GiB): cioè T3
punterebbe gli agenti esattamente alla watermark del container, e con un solo thread pesante il
reclaim comincerebbe a mordere **anche Ambrogio**.

E c'è un secondo dettaglio, che è la ragione per cui il cap non si mette nel drop-in del servizio:

> **gli agenti non girano nel cgroup del servizio.** Ogni agente viene lanciato in uno scope dentro
> `app-t3code-agents.slice` (`apps/server/src/process/agentScope.ts`), che è una slice **sorella** di
> `t3code.service`. Un `MemoryMax=` nel drop-in del servizio non li toccherebbe.

Quindi il cap sta sul **padre**: `systemd/app-t3code.slice`, più lo spostamento del servizio dentro
quella slice. La gerarchia dei nomi delle slice non è un'opinione, è stata verificata qui:

```
$ systemd-run --user --scope --slice=app-t3code-labtest.slice -- /bin/true
$ systemctl --user show app-t3code-labtest.slice -p Slice
Slice=app-t3code.slice          # il nome crea la gerarchia: app.slice > app-t3code.slice > app-t3code-* ...
$ systemctl --user set-property --runtime app-t3code-labtest.slice MemoryHigh=128M MemoryMax=192M
$ systemctl --user show app-t3code-labtest.slice -p MemoryMax -p MemoryHigh
MemoryHigh=134217728
MemoryMax=201326592             # set-property --runtime funziona sulle slice ed e' quello che usa T3
```

Il cap proposto (nel file, commentato riga per riga):

```
MemoryHigh=2G   MemoryMax=3G   MemorySwapMax=0   CPUQuota=400%   IOWeight=200   TasksMax=1024
```

cioè: T3 Code (server + tutti gli agenti) si prende al massimo 3 GiB degli 8 della macchina, metà
delle 8 CPU, e lascia il resto ad Ambrogio. `TasksMax=1024` è la rete contro il thread che sfugge.
Sono numeri di **partenza**, da alzare solo con `memory.high` del container davanti.

### 5.2 Il disco, e `t3 doctor`

- `df -h /` su CT106 nella sandbox riporta **69 GB, 33 GB liberi (52% usato)**: c'è spazio.
- `t3 doctor` serve a recuperare spazio da un database vecchio: ricostruisce il database passando da
  **una copia temporanea**, quindi vuole **circa il doppio della dimensione del database due volte**:
  una in `TMPDIR` (`/tmp`) e una accanto al database (`~/.t3/userdata`). Il layer imposta
  `TMPDIR=~/.t3/tmp` nel drop-in proprio perché quel controllo cada sullo stesso filesystem, e
  `verify-t3code.sh` calcola lo spazio libero **rispetto alla dimensione reale di `statev2.sqlite`**
  (`~/.t3/userdata/statev2.sqlite`, nome verificato in `apps/server/src/config.ts:124`).
- **Ma in 0.1.0 `t3 doctor` non esiste** (verificato eseguendolo). Nella versione stabile la
  manutenzione è automatica ("T3 Code removes superseded internal history on its own while it
  runs"); il comando compare nel repository `main` ed è destinato alle release successive. La
  proposta quindi: (a) non fa dipendere la procedura da `t3 doctor`, (b) lascia il controllo dello
  spazio in `verify-t3code.sh`, (c) segnala di riesaminare il punto dopo il primo `t3 update`.

### 5.3 La garbage collection dei worktree, e il `.env`

Confermato nei docs e va tenuto presente: la politica **`Keep worktrees with local changes` =
`Uncommitted changes`** protegge i file tracciati ma **cancella i file ignorati, `.env` compreso**.
L'alternativa `Any local files` li protegge (tranne `node_modules`). Indicazione per la prova: sul
progetto di T3 mettere **`Any local files`** e una retention esplicita, oppure — meglio — tenere il
`.env` **fuori dal worktree** con un symlink creato dal hook `runOnWorktreeCreate`
(`setup-worktree.sh` in questa cartella), che è la soluzione che la guida stessa suggerisce e che
qui è verificata end-to-end:

```
$ git worktree add ../wt -b t3/demo          # il worktree NON ha .env (.gitignore)
$ T3CODE_PROJECT_ROOT=/repo bash setup-worktree.sh
linked .env -> /repo/.env
linked .env.local -> /repo/.env.local
worktree ready: ... (2 link(s))
$ # seconda esecuzione: 0 link, nessuna sovrascrittura   (idempotente)
$ # scrivendo in /repo/.env, il worktree legge subito il valore nuovo
```

### 5.4 Le trappole: quali valgono, quali no

| Trappola | Verdetto su questa macchina |
|---|---|
| `Linger` disabilitato → il servizio muore alla chiusura dell'SSH | **Vera e misurata**: `Linger=no`. Il layer lo abilita e lo verifica |
| `IS_SANDBOX=1` serve per far girare Claude in LXC | **Falsa come e' scritta** — vedi §7. Serve solo se si fa girare Claude **come root** con `--dangerously-skip-permissions`, e non disattiva nessuna sandbox di Claude |
| Il servizio non trova le CLI dei provider | **Vera**, per una ragione diversa da quella della guida: non è il container, è il **PATH** del gestore utente che non contiene `~/.local/bin` (misurato) |
| Lock orfano / "already running" dopo un riavvio non pulito | **Plausibile**, e il file esiste davvero (`~/.t3/userdata/server-runtime.json`, pid + origin). `ExecStartPre` lo sposta da parte **solo se il pid non è vivo** |
| GC dei worktree che cancella i file ignorati (`.env`) | **Vera** (docs), mitigata dal symlink del hook |
| `t3 doctor` vuole il doppio dello spazio | **Vero nei docs, non applicabile in 0.1.0**: il comando non c'è. Lo spazio si controlla comunque |
| Agenti CLI zombie senza `SIGTERM` | **Non verificata**: né i docs né il codice la documentano come comportamento noto. Non ci si costruisce sopra niente; il cap di memoria e `TasksMax` limitano il danno |
| Browser headless che non parte su Ubuntu 24.04 | **Da aspettarsi qui**: Ubuntu 24.04 limita gli user namespace non privilegiati; il rimedio ufficiale è `sudo t3 browser setup`, il ripiego `T3CODE_SERVER_BROWSER_SANDBOX=0`. `verify-t3code.sh` lo segnala prima della prova |
| Notifiche push senza T3 Connect | **Vera** (docs) |

---

## 6. La prova che l'operatore vuole fare

### 6.1 Accoppiare laptop e telefono

```bash
# 0. su CT106: tailnet a posto e servizio su
sudo tailscale up --ssh --hostname=ct106 && tailscale ip -4
t3 service status

# 1. pubblicare sulla tailnet e prendere il link di pairing (una volta sola)
t3 pair --tailscale                 # -> https://ct106.<tailnet>.ts.net  + QR
#    per la sola LAN, in casa:  t3 pair        (link su 192.168.1.206)
```

- **Laptop**: app desktop (`sudo apt install ./T3-Code-*.deb`) → `Settings → Connections` →
  spegnere **Local environment** (controller puro) → **Add environment** col link → il progetto
  appare.
- **Telefono**: app dallo store → `Add environment` (o scansione del QR) → l'ambiente è salvato e
  resta accoppiato; un nuovo device vuole un link nuovo.
- **Da fuori**: si può togliere il Wi-Fi e lasciare il telefono in LTE — l'app ricade sulla rotta
  tailnet da sola, e il lavoro sul server non si interrompe.

### 6.2 Il primo task, su un repository vero

Sul **clone dedicato** (non sul checkout di Ambrogio) del repository `tazpod`:

```bash
git clone git@github.com:tazzo/tazpod.git ~/t3lab/tazpod     # clone separato: vedi §4.2
cp <questo layer>/t3.json                    ~/t3lab/tazpod/t3.json
mkdir -p ~/t3lab/tazpod/.t3
cp <questo layer>/setup-worktree.sh          ~/t3lab/tazpod/.t3/setup-worktree.sh
git -C ~/t3lab/tazpod add t3.json .t3/setup-worktree.sh && git -C ~/t3lab/tazpod commit -m "t3: worktree setup"
```

Poi, dall'app: *New thread* → progetto `tazpod` → **New worktree** → provider **OpenCode** →
permission mode **Supervised** (primo giro: si vuole vedere ogni scrittura e ogni comando) → task:

> *"Aggiungi il sottocomando `tazpod version` che stampa il contenuto di `VERSION`, con lo stile degli
> altri comandi in `cmd/tazpod/`, e aggiorna il README. Non toccare altro."*

E' piccolo, vero, verificabile, e tocca CLI, docs e worktree.

### 6.3 Cosa guardare, e come si capisce se lo strumento serve

Durante il primo task:

1. **il setup del worktree**: i cinque stadi (`fetch`, `checkout`, `submodules`, `setup-script`,
   `agent`) devono comparire e il `setup-script` deve stampare i due link;
2. **il provider**: OpenCode trovato, modello scelto, nessun `provider not found`;
3. **le approvazioni**: ogni scrittura/comando deve arrivare come richiesta — se non arrivano, la
   permission mode non è quella che si crede;
4. **la memoria**: `systemd-cgtop` (o `systemctl --user show app-t3code.slice -p MemoryCurrent`)
   durante il task: deve restare sotto `MemoryMax=3G`;
5. **il telefono**: il diff si legge davvero? il pollice approva e rifiuta senza attriti?
6. **le notifiche**: **non arriveranno** — è il prezzo della rotta tailnet (§2.1). Se l'operatore
   scopre che senza notifiche lo strumento non gli serve, la risposta è T3 Connect, con il prezzo
   opposto;
7. **la rete che cambia**: passare da Wi-Fi a LTE a metà task: il thread deve continuare, e solo il
   client deve ricadere sull'altra rotta.

Tre domande per decidere se T3 Code merita un posto fisso:

- *il telefono è un posto dove lavoro davvero* (leggere un diff e approvarlo) o solo un posto dove
  guardo?
- *il turno interattivo* mi serve più della coda — cioè: quante volte vorrei "entrare adesso in
  questo task" invece di aprire l'ennesima issue?
- il **costo di gestione** (una superficie in più, agenti con login separati, un altro SQLite, un
  altro posto dove guardare) è giustificato da quanto sopra?

Se le prime due risposte sono sì e la terza è "sì, e lo tengo su un repository suo", la prova è
riuscita.

---

## 7. Cosa della guida non torna (verificato il 2026-10-17)

| # | La guida dice | Verdetto | Prova |
|---|---|---|---|
| 1 | `IS_SANDBOX=1` «segnala a Claude di aggirare i propri controlli di isolamento, permettendo al processo di operare usando i confini del container come unica barriera» | **Falso come e' scritto.** `IS_SANDBOX` è una variabile di **Claude Code**, non di T3 Code (`grep IS_SANDBOX` sulla sorgente T3: nessun risultato). Serve a **sbloccare `--dangerously-skip-permissions` quando Claude gira come root** (`uid 0`), e non disattiva nessuna sandbox di Claude. Con il servizio su un utente non-root non serve a niente; metterlo "per sicurezza" indebolisce proprio la protezione che si voleva ottenere | sorgente T3 senza occorrenze; docs Claude Code (`IS_SANDBOX`: "signals that code is running inside a sandbox") + issue `anthropics/claude-code#3490`, `#58197` (root + `bypassPermissions` → exit 1, escape hatch `IS_SANDBOX=1`) |
| 2 | «T3 Code richiede Node.js 22.16 o superiore» come requisito dell'installazione | **Impreciso.** L'installer non usa Node (dichiara `no Node, npm, or compiler` e non chiama `npm`); Node serve per la via `npx` e per il provider Pi (>= 22.19). Su CT106 c'è comunque Node 24.18.0 | testa di `https://t3.codes/install.sh`; installazione eseguita qui senza problemi |
| 3 | Drop-in systemd in `/etc/systemd/system/t3code.service.d/50-cpu.conf` per limitare CPU/memoria | **Percorso sbagliato**, ed effetto incompleto. `t3code.service` è un servizio **utente** → `~/.config/systemd/user/t3code.service.d/`. E soprattutto: un limite sul servizio **non copre gli agenti**, che girano in scope dentro `app-t3code-agents.slice`, sorella del servizio. Il limite va sul **padre** `app-t3code.slice` | unit generata in `apps/server/src/cloud/bootService.ts`; `apps/server/src/process/agentScope.ts`; gerarchia delle slice verificata con `systemctl --user show ... -p Slice` |
| 4 | `t3 doctor` per fare manutenzione del database (con la trappola del doppio spazio) | **Vero nei docs, non nella release installata.** In `0.1.0` il comando non esiste: `t3 doctor --dry-run` → `Unrecognized flag: --dry-run in command t3` | eseguito sulla release 0.1.0 installata qui |
| 5 | Elenco provider: «Claude Code, Codex, Cursor, Grok Build, OpenCode, Muse Code, Pi, Google Antigravity» | **La guida è più aggiornata della ricerca che accompagna la richiesta** (che ne elencava sei): la documentazione upstream corrente elenca otto provider, Muse Code e Pi compresi | `docs/user/install.md` §Providers; `README.md` |
| 6 | Esempio di `t3.json` con `"id": "setup-worktree-environment"` | **Campo inesistente.** Lo schema pubblicato non ha `id` (`additionalProperties: false`): i campi sono `name`, `command`, `icon`, `runOnWorktreeCreate`, `runOnSettle`, `async`, `previewUrl`, `autoOpenPreview`; l'icona `"configure"` è valida, ma le icone ammesse sono `play, test, lint, configure, build, debug` | `https://t3.codes/schema/t3.json` confrontato campo per campo; il `t3.json` di questo layer lo valida senza errori |
| 7 | `$T3CODE_PROJECT_ROOT` | **Vero**: il server lo inietta nei comandi `runOnWorktreeCreate` col percorso del checkout primario | `packages/shared/src/projectScripts.ts:56`; `scripts/setup-worktree.ts:8` |
| 8 | «Il TLS viene terminato sui server Cloudflare» per T3 Connect | **Vero**, con una sfumatura: dopo il bootstrap il relay **non** fa da proxy alle sessioni HTTP/WebSocket; è però un **broker fidato** che firma le credenziali di bootstrap dell'ambiente | `docs/internals/t3-connect.md` |
| 9 | Android: notifiche push solo con T3 Connect | **Vero** | `docs/user/mobile-notifications.md`: *"Background delivery requires T3 Connect"* |
| 10 | Il documento della guida contiene artefatti di generazione (`[span_65](start_span)`), tabelle spezzate e il capitolo 8 mutilato | **Vero**: è materiale generato, come sospettava la richiesta. Va letto come traccia, non come fonte | il testo archiviato nel progetto |

---

## 8. Cosa ho deciso io, e perché

1. **Metodo di installazione: installer ufficiale come utente, mai root.** Perché: è l'unica via che
   non richiede Node, è quella che l'upstream supporta per il servizio (`t3 service install`), e
   perché da root Claude Code rifiuta la modalità che T3 usa di default (`Full access`).
2. **Cap delle risorse sul padre `app-t3code.slice`, non sul servizio.** Perché gli agenti vivono in
   una slice sorella e T3 ne sovrascrive i limiti a runtime: solo il padre tiene.
3. **`PATH` esplicito nel drop-in.** Perché senza quello, su questa macchina, T3 Code non trova
   nemmeno un provider — ed è un guasto che sembra "T3 non vede OpenCode", non un problema di
   ambiente.
4. **Rotta Tailscale, non T3 Connect, per la prova.** Perché il laboratorio ha già Tailscale, e
   perché il prezzo (niente push) è più accettabile del contrario (TLS terminato da un terzo e relay
   fidato) per un primo esperimento sui propri repository.
5. **Ambrogio e T3 su repository diversi nella prova.** Perché l'unica cosa che due orchestratori
   non possono condividere senza conflitto è la **corsia di pubblicazione**, e nel laboratorio quella
   corsia ha un cancello umano che non va aggirato dal bottone di un telefono.
6. **Il file guida non viene corretto.** La richiesta dice che dove guida e ricerca non concordano
   vale la ricerca: le divergenze stanno qui (§7) e diventano verifiche eseguibili
   (`verify-t3code.sh`), invece di una riscrittura del documento dell'operatore.

## 9. Le ricerche

Primarie (lette direttamente, 2026-10-17):

- repository: <https://github.com/pingdotgg/t3code> (MIT; albero `main` scaricato e interrogato)
- installazione: <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/user/install.md>;
  installer: <https://t3.codes/install.sh>
- servizio in background e manutenzione DB:
  <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/user/background-service.md>
- accesso remoto, Tailscale, pairing:
  <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/user/remote-access.md>
- notifiche mobile: <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/user/mobile-notifications.md>
- T3 Connect, architettura: <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/internals/t3-connect.md>
- agenti esterni via MCP:
  <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/user/outside-agents.md>
- modalità di permesso: <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/user/permission-modes.md>
- impostazioni di progetto, cleanup, `t3.json`:
  <https://raw.githubusercontent.com/pingdotgg/t3code/main/docs/user/project-settings.md>
- schema di `t3.json`: <https://t3.codes/schema/t3.json>
- codice citato: `apps/server/src/cloud/bootService.ts`, `apps/server/src/process/agentScope.ts`,
  `apps/server/src/config.ts`, `packages/contracts/src/t3ProjectFile.ts`,
  `packages/shared/src/projectScripts.ts`, `apps/server/src/vcs/VcsProcess.ts`
- Claude Code (`IS_SANDBOX`): <https://code.claude.com/docs/en/sandbox-environments>,
  <https://github.com/anthropics/claude-code/issues/3490>, `anthropics/claude-code#58197`

Secondarie (guide della guida dell'operatore, non usate come fonte): opensourcealternatives,
paralect, flaviocopes, codeagentswarm, Play Store, codecast.

## 10. I file di questo layer

| File | Cosa fa |
|---|---|
| `PROPOSTA.md` | questo documento |
| `README.md` | indice della cartella e sequenza di esecuzione |
| `install-t3code.sh` | installer root-side, idempotente: linger → CLI → drop-in → servizio → verifica |
| `verify-t3code.sh` | preflight e verifica: `PASS`/`FAIL` su linger, gestore utente, PATH dei provider, tailnet, porta/bind, disco vs database, cap, browser |
| `uninstall-t3code.sh` | rimozione ordinata del servizio e dei drop-in (`--purge` cancella anche `~/.t3`) |
| `systemd/t3code.service.d/50-lab.conf` | template del drop-in del servizio: slice, `PATH`, `TMPDIR`, guardia al file di runtime |
| `systemd/app-t3code.slice` | template del cap di risorse (server + agenti) |
| `systemd/clean-stale-runtime.sh` | `ExecStartPre`: sposta da parte `server-runtime.json` **solo** se il pid non è vivo |
| `check-layer.sh` | controlli statici offline: sintassi degli script e validità di `t3.json` contro lo schema pubblicato |
| `t3.json` | file di progetto per il repository obiettivo (validato contro lo schema pubblicato) |
| `setup-worktree.sh` | hook `runOnWorktreeCreate`: collega `.env`/`.env.local` dal checkout primario, idempotente |
| `documenti/` | piano, roadmap e progresso del progetto, piu' la copia dei documenti pubblicati sul hub (`pubblicati.json`) e lo snapshot dello schema (`t3.schema.json`) |
| `RAPPORTO.md` | la ricostruzione del lavoro: cosa e' stato fatto, come, perche', con le misure |

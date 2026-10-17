# Progress (Progresso) — passo 89: la proposta per T3 Code in laboratorio

**Data**: 2026-10-17 · **Stato**: consegnato · **Prossimo**: decisione dell'operatore (R1).

## Fatto in questo passo

- **Ricerca primaria** su `pingdotgg/t3code` (branch `main`, albero completo) e sulla documentazione
  in-repo (`docs/user/*`, `docs/operations/*`, `docs/internals/*`), piu' la guida dell'operatore letta
  dal progetto.
- **Prova d'installazione** della release stabile in una cartella di lavoro: `t3 v0.1.0`,
  installer ufficiale, `--help` di `serve`/`pair`/`service`/`connect`/`auth`/`browser` letti e citati.
- **Misure sulla macchina** (dalla sandbox del worker, quindi da riconfermare con root):
  `Linger=no`; il PATH del gestore utente non contiene `~/.local/bin` (dove stanno `opencode` e `pi`);
  `tailscale status` risponde `Logged out`; la gerarchia delle slice systemd e
  `set-property --runtime` verificati con una slice di prova poi rimossa; il bind di qualunque porta
  e' negato dentro la sandbox (percio' il server non si puo' provare qui).
- **File prodotti** in `runtimes/lxc/t3code/`: installer idempotente, verifica, rimozione, i due
  template systemd (drop-in del servizio e slice con il cap), la guardia al file di runtime orfano,
  il `t3.json` di progetto e l'hook `runOnWorktreeCreate`.
- **Verifiche eseguite sui file**: `bash -n`/`sh -n` su tutti gli script; `t3.json` validato campo per
  campo contro lo schema pubblicato (`additionalProperties: false`); l'hook del worktree provato su un
  repository di prova (symlink dei file ignorati, idempotenza, propagazione della modifica);
  la guardia del file di runtime provata con un pid morto e con un pid vivo.
- **Pubblicazione** della proposta e dei documenti di progetto sul hub del progetto `t3code`
  (id 127–130). I tre documenti di coda sono stati poi **ripubblicati con il titolo in inglese**
  (`Plan …` id 131, `Roadmap …` id 132, `Progress …` id 133): il controllo della coda cerca i
  documenti per parola — "plan", "roadmap", "progress" — e i titoli italiani non li soddisfacevano.
  Nel repository i file si chiamano `documenti/PLAN.md`, `documenti/ROADMAP.md`,
  `documenti/PROGRESS.md` per la stessa ragione. Le copie con il titolo italiano (128–130) restano
  sul hub: i documenti non si cancellano da qui.

## Cosa resta aperto (e non e' di questo passo)

1. **Installazione vera su CT106**: richiede root sul container e la chiave `tazpod-provision`.
2. **Tailnet**: il nodo deve tornare nel tailnet per la rotta consigliata.
3. **Login dei provider** che l'operatore vuole accendere oltre a OpenCode, Pi e Antigravity.
4. **La prova** laptop/telefono su un repository vero, e la decisione sulla convivenza con Ambrogio.

## Cosa questo passo NON ha potuto verificare, e perche'

- **Il server in esecuzione**: la sandbox del worker vieta il bind di qualunque porta (`EACCES` anche
  su `127.0.0.1:0`). Il binario parte (banner del runtime, versione) e arriva a `ServeError` solo per
  quel divieto. La prova di ascolto va fatta su CT106 dopo l'installazione: e' la prima riga di
  `verify-t3code.sh`.
- **I numeri interni del container** (`memory.high`/`memory.max`): `/sys/fs/cgroup` e' negato alla
  sandbox; la proposta usa i valori dichiarati dall'operatore (6 GiB / 8 GiB) e la verifica li rilegge
  da root.
- **I bug "di esercizio" citati dalla guida** (agenti CLI zombie, fallimenti OAuth dell'adapter MCP):
  non sono documentati upstream, quindi la proposta non ci costruisce sopra — li elenca come non
  verificati.

## Dove sono le cose

- Proposta e layer: `runtimes/lxc/t3code/` (`PROPOSTA.md`, `README.md`, `install-t3code.sh`,
  `verify-t3code.sh`, `uninstall-t3code.sh`, `systemd/`, `t3.json`, `setup-worktree.sh`).
- Documenti di progetto: `runtimes/lxc/t3code/documenti/` (questo file, `PLAN.md`, `ROADMAP.md`) e il
  hub del progetto `t3code`.

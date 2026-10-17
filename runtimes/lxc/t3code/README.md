# `t3code` — il layer che porta T3 Code in CT106

T3 Code (`pingdotgg/t3code`, MIT) è una *control surface*: un server Node con WebSocket che governa le
CLI degli agenti di codifica gia' presenti sulla macchina, con client web, desktop Electron e mobile.
Questo layer lo installa **dentro CT106** (`tazpod-proxmox`, 192.168.1.206) come servizio utente
systemd, con un cap di risorse, e lo rende comandabile da fuori.

**La proposta è `PROPOSTA.md`**: leggete quella. Questa cartella e' la sua parte eseguibile.

## Sequenza

```bash
# 0. preflight: dice cosa manca PRIMA di installare (linger, PATH, tailnet, disco, bind)
ssh root@192.168.1.206 'T3_USER=tazpod bash -s' < verify-t3code.sh

# 1. installa (idempotente): linger, CLI, drop-in, servizio, verifica finale
ssh root@192.168.1.206 'T3_USER=tazpod bash -s' < install-t3code.sh

# 2. esegui install-t3code.sh copiando anche systemd/ accanto allo script
#    (i template vengono resi da li'), oppure copia la cartella intera:
scp -r . root@192.168.1.206:/root/t3code
ssh root@192.168.1.206 'T3_USER=tazpod bash /root/t3code/install-t3code.sh'

# 3. accoppia laptop e telefono (una volta sola)
#    su CT106 come tazpod:
t3 pair --tailscale

# 4. rimozione ordinata
ssh root@192.168.1.206 'bash -s' < uninstall-t3code.sh          # --purge cancella anche ~/.t3
```

## File

| File | Ruolo |
|---|---|
| `PROPOSTA.md` | la proposta completa: installazione, rotta da fuori, provider, Ambrogio, limiti, trappole, prova |
| `install-t3code.sh` | installer root-side idempotente |
| `verify-t3code.sh` | preflight e verifica (`PASS`/`FAIL`, esce 1 se qualcosa fallisce) |
| `uninstall-t3code.sh` | rimozione di servizio e drop-in |
| `systemd/t3code.service.d/50-lab.conf` | template del drop-in del servizio: `Slice`, `PATH`, `TMPDIR`, guardia anti-lock |
| `systemd/app-t3code.slice` | template del cap di risorse (server **e** agenti) |
| `systemd/clean-stale-runtime.sh` | `ExecStartPre` per il file di runtime orfano |
| `t3.json` | file di progetto per il repository obiettivo |
| `setup-worktree.sh` | hook `runOnWorktreeCreate` (symlink dei file ignorati) |
| `documenti/` | `PLAN.md`, `ROADMAP.md`, `PROGRESS.md` del progetto, `check-documents.sh` (verifica che il hub li porti), `pubblicati.json` (snapshot), `t3.schema.json` (schema di `t3.json`) |

I due template in `systemd/` contengono i segnaposto `@T3_USER@`, `@T3_USER_HOME@`, `@NODE_BIN_DIR@`:
li sostituisce `install-t3code.sh` scrivendo in `~/.config/systemd/user/` dell'utente di servizio.

## Regola che vale per tutto il layer

Il server, gli agenti e le CLI dei provider girano **come utente di servizio, mai come root**: da root
Claude Code rifiuta la modalità di permessi che T3 Code usa di default, e `IS_SANDBOX=1` (la variabile
della guida) e' un aggiramento di quel controllo, non una configurazione per LXC. Vedi `PROPOSTA.md` §7.

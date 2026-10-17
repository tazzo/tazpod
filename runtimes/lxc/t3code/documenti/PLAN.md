# Plan (Piano) — T3 Code in laboratorio (CT106)

**Progetto**: `t3code` · **Aperto**: 2026-10-17 · **Origine**: richiesta dell'operatore del
2026-10-17 («installarlo sul laptop e comandare i miei agenti... lavorano nel container 106... provare
anche l'applicazione, soprattutto da fuori con laptop o cellulare») + la sua
*Guida — Installazione T3 Code in laboratorio*.

## Obiettivo

Portare **T3 Code** in CT106 (`tazpod-proxmox`, 192.168.1.206) come *control surface* degli agenti di
codifica che gia' girano sulla macchina, comandabile da laptop, telefono e browser **da fuori**, e
decidere — con una prova vera — se lo strumento merita un posto stabile accanto ad Ambrogio.

## Che cosa e' T3 Code (una riga)

Un server Node con WebSocket che governa le CLI degli agenti (Codex, Claude, Cursor, Grok,
OpenCode, Antigravity, Pi, Muse) e le espone a tre client — web, desktop Electron, mobile — con
isolamento per thread in git worktree; non e' un agente e non rivende modelli (BYOK).

## Confini espliciti

- **Nel perimetro**: installazione in CT106, servizio utente systemd con `Linger`, cap di risorse,
  rotta di accesso remoto, abilitazione dei provider presenti, la convivenza con Ambrogio, la prova.
- **Fuori dal perimetro**: toccare il motore Ambrogio o la coda; pubblicare qualunque cosa; cambiare
  la configurazione di Tailscale della lab; adottare T3 Connect (Cloudflare) — che resta una scelta
  dell'operatore, con il prezzo dichiarato (TLS terminato da un terzo, relay fidato).
- **Non negoziabile**: il servizio gira come utente non privilegiato, mai root; nessuna azione
  distruttiva sui repository; nessun segreto in Git.

## Criteri di riuscita

1. `runtimes/lxc/t3code/` contiene un'installazione eseguibile (installer, servizio, drop-in, cap,
   `t3.json`, hook del worktree) e `verify-t3code.sh` dice `PASS` su CT106.
2. Il servizio sopravvive alla chiusura della sessione SSH (`Linger=yes`) e al riavvio del container.
3. Laptop e telefono accoppiati comandano un thread su un repository **vero**, con worktree creato e
   agente che lavora.
4. La risposta alla domanda "Ambrogio e T3 insieme" e' scritta, motivata e verificabile
   (repository diversi per la prova, collisioni nominate, ponte MCP citato).
5. Il costo di risorse e' limitato da un cap che il container puo' permettersi.

## Vincoli della macchina

- CT106: `memory.high = 6 GiB`, `memory.max = 8 GiB`, 8 CPU; ci girano gia' il motore Ambrogio,
  `opencode`, `pi`, `omp` (`Linger=no`, misurato).
- L'incidente di saturazione del 2026-10-02 (`memory/reports/`) e' la ragione per cui il cap non e'
  facoltativo.
- La release stabile di T3 Code e' **0.1.0** e la sua documentazione e' piu' avanti di lei: le
  funzioni usate devono esistere in 0.1.0, non solo in `main`.

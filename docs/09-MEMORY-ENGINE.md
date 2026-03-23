# Mnemosyne: Semantic Memory Engine 🧠

TazPod includes a powerful semantic memory engine called **Mnemosyne**. It allows the environment to "remember" technical decisions, debugging steps, and architecture patterns by ingesting session logs and indexing them into a vector database (PostgreSQL with `pgvector`).

## 1. Overview

Mnemosyne extracts "High-Resolution" technical chronicles from your development sessions. Each chronicle is structured to include:
*   **[PROBLEM]**: What was being solved.
*   **[INVESTIGATION]**: What was checked or tried.
*   **[FAILURES]**: What didn't work and why.
*   **[SOLUTION]**: The final working configuration or commands.

These memories are vectorized using Google's `text-embedding-004` model and stored in a local PostgreSQL instance within the TazLab cluster.

## 2. CLI Commands

You can interact with the memory engine directly via the `tazpod` CLI.

### `tazpod memory sync [dir]`
Synchronizes all session logs (`.md` files) in the specified directory (defaults to current) with the semantic database.
*   It automatically handles VPN connectivity to the database cluster. (**Note**: VPN command is currently UNTESTED and has not been validated.)
*   It skips already processed files using an `archived_files` tracking table.
*   It performs "Deep Sniffing" to exclude meta-sessions (sessions about archiving themselves).

```bash
tazpod memory sync /workspace/chats/md/
```

### `tazpod memory next <dir>`
Ingests only the **next** unprocessed file in the directory. This is useful for testing or incremental processing.

```bash
tazpod memory next /workspace/chats/md/
```

### `tazpod memory wipe`
Clears all memories and the archive tracker from the database. Use with caution.

```bash
tazpod memory wipe
```

## 3. Configuration

Mnemosyne relies on the `providers` configuration in your `.tazpod/config.yaml`:

```yaml
active_provider: home
providers:
  home:
    db_host: "192.168.1.241"
    vpn_config: "HOME_WG_CONF" # Secret name containing Wireguard config
```

The engine automatically brings up the VPN before connecting to the database and tears it down afterwards.

## 4. Under the Hood

The engine uses a Python-based core (`mnemosyne.py`) which interfaces with:
1.  **Gemini CLI**: For high-speed, cost-effective fact extraction.
2.  **pgvector**: For efficient semantic similarity searches.
3.  **Chronicler**: A pre-processor that cleans logs, removes noise, and ensures high SNR (Signal-to-Noise Ratio).

---
*Next: Learn about the Nomadic Identity sync in [08-NOMADIC-IDENTITY.md](./08-NOMADIC-IDENTITY.md)*

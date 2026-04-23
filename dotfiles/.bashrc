# ~/.bashrc: executed by bash(1) for non-login shells.

# If not running interactively, don't do anything
case $- in
*i*) ;;
*) return ;;
esac

HISTCONTROL=ignoreboth
shopt -s histappend
HISTSIZE=1000
HISTFILESIZE=2000
shopt -s checkwinsize

[ -x /usr/bin/lesspipe ] && eval "$(SHELL=/bin/sh lesspipe)"

if [ -z "${debian_chroot:-}" ] && [ -r /etc/debian_chroot ]; then
  debian_chroot=$(cat /etc/debian_chroot)
fi

# --- PATH ENHANCEMENTS ---
export PATH="$HOME/.local/bin:$PATH"

# --- NVM (Node Version Manager) ---
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"

# Aliases - General
alias ..="cd .."
alias ...="cd ../.."
alias v="nvim"
alias vi="nvim"
alias vim="nvim"

# Aliases - Git
alias g="git"
alias lg="lazygit"
alias gs="git status"
alias gp="git push"
alias gl="git log --oneline --graph --decorate"

# Aliases - DevOps
alias k="kubectl"
alias ctx="kubectx"
alias ns="kubens"
alias tf="terraform"

# Aliases - Modern Tools
alias ls="eza --icons"
alias ll="eza -lh --icons --grid"
alias la="eza -a --icons"
alias lt="eza --tree --icons"
alias l="eza -l --icons --git --no-user --no-time"
alias cat="bat"

# --- TAZPOD CORE (Smart Function v7.2) ---
tazpod() {
    if [ "$1" == "env" ]; then
        eval "$(command tazpod __internal_env 2>/dev/null)"
        echo "🔄 Enclave environment variables refreshed."
        return 0
    fi

    command tazpod "$@";
    local res=$?;
    
    # Auto-reload env on key commands
    if [ "$1" == "unlock" ] || [ "$1" == "pull" ] || [ "$1" == "sync" ] || [ "$1" == "login" ] || [ "$1" == "lock" ]; then
        if [ "$1" == "lock" ]; then sleep 0.1; fi
        eval "$(command tazpod __internal_env 2>/dev/null)"
        if [ "$1" == "lock" ]; then
             echo "🔒 Enclave environment cleaned."
        else
             echo "🔄 Environment updated."
        fi
    fi
    return $res;
}

# Auto-load secrets if already mounted
if mountpoint -q /home/tazpod/secrets; then
    eval "$(command tazpod __internal_env 2>/dev/null)"
    setup_oci_config
fi

# --- AI TOOL CONFIG SYMLINKS (persistent, no unlock required) ---
if [ -d /workspace/.tazpod ]; then
    for _tool in .pi .omp .gemini .claude; do
        _target="/workspace/.tazpod/$_tool"
        _link="$HOME/$_tool"
        mkdir -p "$_target"
        # Recreate if missing, dangling, or pointing to wrong target
        if [ ! -L "$_link" ] || [ "$(readlink "$_link")" != "$_target" ]; then
            rm -rf "$_link" && ln -sf "$_target" "$_link"
        fi
    done
    unset _tool _target _link

    # AWS config: symlink ~/.aws -> /workspace/.tazpod/.aws
    # Skip if already bind-mounted from the vault enclave (vault unlocked)
    if ! mountpoint -q "$HOME/.aws" 2>/dev/null; then
        mkdir -p /workspace/.tazpod/.aws
        if [ ! -L "$HOME/.aws" ] || [ "$(readlink "$HOME/.aws")" != "/workspace/.tazpod/.aws" ]; then
            rm -rf "$HOME/.aws" && ln -sf /workspace/.tazpod/.aws "$HOME/.aws"
        fi
    fi

    # OpenCode: persist config, auth, sessions, and state in workspace
    _opencode_root="/workspace/.tazpod/.opencode"
    mkdir -p "$_opencode_root/config" "$_opencode_root/data" "$_opencode_root/state" "$_opencode_root/cache"

    mkdir -p "$HOME/.config" "$HOME/.local/share" "$HOME/.local/state" "$HOME/.cache"
    if [ ! -L "$HOME/.config/opencode" ] || [ "$(readlink "$HOME/.config/opencode")" != "$_opencode_root/config" ]; then
        rm -rf "$HOME/.config/opencode" && ln -sf "$_opencode_root/config" "$HOME/.config/opencode"
    fi
    if [ ! -L "$HOME/.local/share/opencode" ] || [ "$(readlink "$HOME/.local/share/opencode")" != "$_opencode_root/data" ]; then
        rm -rf "$HOME/.local/share/opencode" && ln -sf "$_opencode_root/data" "$HOME/.local/share/opencode"
    fi
    if [ ! -L "$HOME/.local/state/opencode" ] || [ "$(readlink "$HOME/.local/state/opencode")" != "$_opencode_root/state" ]; then
        rm -rf "$HOME/.local/state/opencode" && ln -sf "$_opencode_root/state" "$HOME/.local/state/opencode"
    fi
    if [ ! -L "$HOME/.cache/opencode" ] || [ "$(readlink "$HOME/.cache/opencode")" != "$_opencode_root/cache" ]; then
        rm -rf "$HOME/.cache/opencode" && ln -sf "$_opencode_root/cache" "$HOME/.cache/opencode"
    fi
    if [ ! -f "$_opencode_root/config/tui.json" ]; then
        cat > "$_opencode_root/config/tui.json" <<'EOF'
{
  "$schema": "https://opencode.ai/tui.json",
  "mouse": false
}
EOF
    fi
    if [ ! -f "$_opencode_root/config/opencode.json" ]; then
        cat > "$_opencode_root/config/opencode.json" <<'OP_EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "opencode-antigravity-auth@beta"
  ],
  "provider": {
    "google": {
      "npm": "@ai-sdk/google",
      "models": {
        "antigravity-gemini-3-pro": {
          "name": "Gemini 3 Pro (Antigravity)",
          "limit": { "context": 1048576, "output": 65535 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] },
          "variants": {
            "low": { "thinkingLevel": "low" },
            "high": { "thinkingLevel": "high" }
          }
        },
        "antigravity-gemini-3.1-pro": {
          "name": "Gemini 3.1 Pro (Antigravity)",
          "limit": { "context": 1048576, "output": 65535 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] },
          "variants": {
            "low": { "thinkingLevel": "low" },
            "high": { "thinkingLevel": "high" }
          }
        },
        "antigravity-gemini-3-flash": {
          "name": "Gemini 3 Flash (Antigravity)",
          "limit": { "context": 1048576, "output": 65536 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] },
          "variants": {
            "minimal": { "thinkingLevel": "minimal" },
            "low": { "thinkingLevel": "low" },
            "medium": { "thinkingLevel": "medium" },
            "high": { "thinkingLevel": "high" }
          }
        },
        "antigravity-claude-sonnet-4-6": {
          "name": "Claude Sonnet 4.6 (Antigravity)",
          "limit": { "context": 200000, "output": 64000 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] }
        },
        "antigravity-claude-opus-4-6-thinking": {
          "name": "Claude Opus 4.6 Thinking (Antigravity)",
          "limit": { "context": 200000, "output": 64000 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] },
          "variants": {
            "low": { "thinkingConfig": { "thinkingBudget": 8192 } },
            "max": { "thinkingConfig": { "thinkingBudget": 32768 } }
          }
        },
        "gemini-2.5-flash": {
          "name": "Gemini 2.5 Flash (Gemini CLI)",
          "limit": { "context": 1048576, "output": 65536 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] }
        },
        "gemini-2.5-pro": {
          "name": "Gemini 2.5 Pro (Gemini CLI)",
          "limit": { "context": 1048576, "output": 65536 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] }
        },
        "gemini-3-flash-preview": {
          "name": "Gemini 3 Flash Preview (Gemini CLI)",
          "limit": { "context": 1048576, "output": 65536 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] }
        },
        "gemini-3-pro-preview": {
          "name": "Gemini 3 Pro Preview (Gemini CLI)",
          "limit": { "context": 1048576, "output": 65535 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] }
        },
        "gemini-3.1-pro-preview": {
          "name": "Gemini 3.1 Pro Preview (Gemini CLI)",
          "limit": { "context": 1048576, "output": 65535 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] }
        },
        "gemini-3.1-pro-preview-customtools": {
          "name": "Gemini 3.1 Pro Preview Custom Tools (Gemini CLI)",
          "limit": { "context": 1048576, "output": 65535 },
          "modalities": { "input": ["text", "image", "pdf"], "output": ["text"] }
        }
      }
    }
  }
}
OP_EOF
    fi

    if [ ! -f "$_opencode_root/config/antigravity.json" ]; then
        cat > "$_opencode_root/config/antigravity.json" <<'AG_EOF'
{
  "$schema": "https://raw.githubusercontent.com/NoeFabris/opencode-antigravity-auth/main/assets/antigravity.schema.json",
  "account_selection_strategy": "sticky",
  "session_recovery": true,
  "quiet_mode": false,
  "debug": false,
  "debug_tui": false,
  "auto_update": true,
  "cli_first": false
}
AG_EOF
    fi

    if [ ! -f "$_opencode_root/config/package.json" ]; then
        echo '{"name":"opencode-config","private":true}' > "$_opencode_root/config/package.json"
    fi

    if [ ! -d "$_opencode_root/config/node_modules/opencode-antigravity-auth" ]; then
        echo "Installing opencode-antigravity-auth plugin in background..."
        (cd "$_opencode_root/config" && npm install opencode-antigravity-auth@beta >/dev/null 2>&1) &
    fi

    unset _opencode_root
fi

# Enable Modern Prompts/Tools
[ -x "$(command -v starship)" ] && eval "$(starship init bash)"
[ -x "$(command -v zoxide)" ] && eval "$(zoxide init bash)"
[ -f ~/.fzf.bash ] && source ~/.fzf.bash

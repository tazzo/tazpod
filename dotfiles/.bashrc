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

# --- INFISICAL CONFIG ---
export INFISICAL_VAULT_BACKEND=file

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
fi

# Enable Modern Prompts/Tools
[ -x "$(command -v starship)" ] && eval "$(starship init bash)"
[ -x "$(command -v zoxide)" ] && eval "$(zoxide init bash)"
[ -f ~/.fzf.bash ] && source ~/.fzf.bash

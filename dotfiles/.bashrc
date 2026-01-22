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

# --- TAZPOD CORE (Smart Function v6.4) ---
tazpod() {
    # Special case for 'env' to prevent leaking secrets to terminal
    if [ "$1" == "env" ]; then
        eval "$(/usr/local/bin/tazpod __internal_env 2>/dev/null)"
        echo "🔄 Enclave environment variables refreshed."
        return 0
    fi

    /usr/local/bin/tazpod "$@";
    local res=$?;
    
    # Outer Shell: Exit on unlock/reinit/pull(if vault was closed)
    if [ -z "$TAZPOD_GHOST_MODE" ]; then
        if [ "$1" == "unlock" ] || [ "$1" == "reinit" ] || [ "$1" == "pull" ]; then
            if [ $res -eq 0 ]; then exit 0; fi;
        fi;
    
    # Inner Ghost Shell: Auto-reload env on sync/login/pull
    else
        if [ "$1" == "pull" ] || [ "$1" == "sync" ] || [ "$1" == "login" ]; then
             eval "$(/usr/local/bin/tazpod __internal_env 2>/dev/null)"
             echo "🔄 Environment updated."
        fi
    fi;
    return $res;
}

# Auto-load secrets on startup if vault is open
if [ -n "$TAZPOD_GHOST_MODE" ]; then
    eval "$(/usr/local/bin/tazpod __internal_env 2>/dev/null)"
fi

# Gemini CLI Safety Wrapper
gemini() {
    if [ "$TAZPOD_GHOST_MODE" = "true" ]; then
        /usr/local/bin/gemini "$@"
    else
        echo -e "\033[0;33m🔒 Vault is closed. Gemini memories are in the secure enclave.\033[0m"
        echo "   Starting unlock procedure... please run 'gemini' again once inside."
        tazpod unlock
    fi
}

# Vault Welcome Message
if [ "$TAZPOD_GHOST_MODE" = "true" ]; then
    echo -e "\n\033[1;32m✅ Vault Unlocked. You can now run 'gemini' safely.\033[0m\n"
fi

# Enable Modern Prompts/Tools
[ -x "$(command -v starship)" ] && eval "$(starship init bash)"
[ -x "$(command -v zoxide)" ] && eval "$(zoxide init bash)"
[ -f ~/.fzf.bash ] && source ~/.fzf.bash
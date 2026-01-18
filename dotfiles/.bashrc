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
alias l="eza -l --icons --git --no-user --no-time" # Compact list
alias cat="bat"

# --- TAZPOD CORE ---
tazpod() {
    /usr/local/bin/tazpod "$@";
    local res=$?;
    if [ -z "$TAZPOD_GHOST_MODE" ]; then
        if [ "$1" == "unlock" ] || [ "$1" == "reinit" ]; then
            if [ $res -eq 0 ]; then exit 0; fi;
        fi;
    fi;
    return $res;
}

# Auto-load secrets if vault is open (Ghost Mode shell)
if [ -n "$TAZPOD_GHOST_MODE" ] && [ -f "$HOME/secrets/.env-infisical" ]; then
    set -a
    source "$HOME/secrets/.env-infisical"
    set +a
fi

# Enable Modern Prompts/Tools
[ -x "$(command -v starship)" ] && eval "$(starship init bash)"
[ -x "$(command -v zoxide)" ] && eval "$(zoxide init bash)"
[ -f ~/.fzf.bash ] && source ~/.fzf.bash

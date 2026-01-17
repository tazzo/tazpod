#!/usr/bin/env python3
import os
import sys
import subprocess
import yaml
import hashlib
import getpass
from pathlib import Path

# --- CONFIGURAZIONE ---
USER_NAME = "tazpod"
HOME_DIR = Path(f"/home/{USER_NAME}")
SECRETS_DIR = HOME_DIR / "secrets"
VAULT_BASE = HOME_DIR / ".vault_persistent"
VAULT_IMG = VAULT_BASE / "vault.img"
VAULT_SIZE_MB = 512
SECRET_NAME_HASH = "TAZPOD_PASSPHRASE_HASH"

def run(cmd, input_str=None, capture=True, sudo=False):
    if sudo:
        cmd = ["sudo"] + cmd
    try:
        res = subprocess.run(
            cmd, 
            input=input_str.encode() if input_str else None,
            capture_output=capture,
            check=True,
            text=True
        )
        return res.stdout.strip()
    except subprocess.CalledProcessError as e:
        return None

def get_secrets_yaml():
    # Cerca secrets.yml nella cartella corrente o in /workspace
    p = Path("./secrets.yml")
    if p.exists(): return p
    p = Path("/workspace/secrets.yml")
    if p.exists(): return p
    return None

def get_config(yaml_path):
    with open(yaml_path, 'r') as f:
        return yaml.safe_load(f)

def get_mapper_name():
    # Nome univoco basato sulla directory corrente per permettere più vault
    cwd = os.getcwd()
    path_hash = hashlib.md5(cwd.encode()).hexdigest()[:8]
    return f"zt_vault_{path_hash}"

def up():
    yaml_path = get_secrets_yaml()
    if not yaml_path:
        print(f"echo '❌ Error: secrets.yml not found in current directory or /workspace' >&2")
        sys.exit(1)

    config = get_config(yaml_path)
    project_id = config.get('config', {}).get('infisical_project_id')
    api_url = config.get('config', {}).get('infisical_url')
    
    if api_url:
        os.environ["INFISICAL_API_URL"] = api_url

    # 1. Check Mount
    if os.path.ismount(str(SECRETS_DIR)):
        load_env()
        return

    # 2. Auth Check
    hash_val = run(["infisical", "secrets", "get", SECRET_NAME_HASH, "--projectId", project_id])

    if not hash_val:
        if not VAULT_IMG.exists():
            print(f"echo '🆕 Setup: Infisical Login required...' >&2")
            subprocess.run(["infisical", "login"], check=True)
            hash_val = run(["infisical", "secrets", "get", SECRET_NAME_HASH, "--projectId", project_id])
        else:
            print(f"echo '⚠️  Offline mode.' >&2")

    # 3. Passphrase Entry
    while True:
        prompt = "🔑 Enter TazPod Master Passphrase: "
        plain_pass = getpass.getpass(prompt)
        
        if hash_val:
            salt = hash_val.split('$')[2]
            check_hash = run(["openssl", "passwd", "-6", "-salt", salt, "-stdin"], input_str=plain_pass)
            if check_hash == hash_val: break
            print(f"echo '❌ Wrong passphrase.' >&2")
        else:
            if not VAULT_IMG.exists():
                confirm = getpass.getpass("📝 Confirm Passphrase: ")
                if plain_pass == confirm:
                    hash_val = run(["openssl", "passwd", "-6", "-stdin"], input_str=plain_pass)
                    run(["infisical", "secrets", "set", f"{SECRET_NAME_HASH}={hash_val}", "--projectId", project_id])
                    break
                print(f"echo '❌ No match.' >&2")
            else: break

    # 4. LUKS Open
    mapper = get_mapper_name()
    VAULT_BASE.mkdir(parents=True, exist_ok=True)
    
    if not VAULT_IMG.exists():
        run(["dd", "if=/dev/zero", f"of={VAULT_IMG}", "bs=1M", f"count={VAULT_SIZE_MB}", "status=none"])
        loop_dev = run(["losetup", "-f", "--show", str(VAULT_IMG)], sudo=True)
        run(["cryptsetup", "luksFormat", loop_dev], input_str=plain_pass, sudo=True)
        run(["cryptsetup", "open", loop_dev, mapper], input_str=plain_pass, sudo=True)
        run(["mkfs.ext4", "-q", f"/dev/mapper/{mapper}"], sudo=True)
    else:
        loop_dev = run(["losetup", "-f", "--show", str(VAULT_IMG)], sudo=True)
        if run(["cryptsetup", "open", loop_dev, mapper], input_str=plain_pass, sudo=True) is None:
            print(f"echo '❌ Decryption failed.' >&2")
            run(["losetup", "-d", loop_dev], sudo=True)
            sys.exit(1)

    # 5. Mount
    SECRETS_DIR.mkdir(parents=True, exist_ok=True)
    run(["mount", f"/dev/mapper/{mapper}", str(SECRETS_DIR)], sudo=True)
    run(["chown", "-R", f"{USER_NAME}:{USER_NAME}", str(SECRETS_DIR)], sudo=True)

    # 6. Sync
    print(f"echo '📦 Syncing secrets...' >&2")
    env_file = SECRETS_DIR / ".env-infisical"
    
    with open(env_file, 'w') as ef:
        gen_export = run(["infisical", "export", "--projectId", project_id, "--format=dotenv"])
        if gen_export: ef.write(gen_export + "\n")
        
        secrets_list = config.get('secrets', [])
        for s in secrets_list:
            name, filename, env_var = s.get('name'), s.get('file'), s.get('env')
            val = run(["infisical", "secrets", "get", name, "--projectId", project_id, "--plain"])
            if val:
                target = SECRETS_DIR / filename
                with open(target, 'w') as sf: sf.write(val)
                target.chmod(0o600)
                if env_var: ef.write(f"export {env_var}=\"{target}\"\n")

    load_env()
    print(f"echo '✅ TAZPOD SECURED.' >&2")

def load_env():
    env_file = SECRETS_DIR / ".env-infisical"
    if env_file.exists():
        with open(env_file, 'r') as f:
            for line in f:
                if line.startswith('export '): print(line.strip())

def down():
    mapper = get_mapper_name()
    if os.path.ismount(str(SECRETS_DIR)):
        run(["umount", "-f", str(SECRETS_DIR)], sudo=True)
    if Path(f"/dev/mapper/{mapper}").exists():
        run(["cryptsetup", "close", mapper], sudo=True)
    subprocess.run(f"sudo losetup -a | grep '{VAULT_IMG}' | cut -d: -f1 | xargs -r sudo losetup -d", shell=True)

if __name__ == "__main__":
    if len(sys.argv) < 2: sys.exit(1)
    if sys.argv[1] == "up": up()
    elif sys.argv[1] == "down": down()
    elif sys.argv[1] == "env": load_env()
import yaml
import subprocess
import os
from pathlib import Path

# Configurazione
BASE_DIR = Path(__file__).parent.resolve()
INDEX_FILE = BASE_DIR / "INDEX.yaml"
MNEMOSYNE_SCRIPT = BASE_DIR / "mnemosyne.py"
OUTPUT_FILE = Path("/workspace/CURRENT_CONTEXT.md")

def awaken():
    if not INDEX_FILE.exists():
        print(f"❌ Errore: {INDEX_FILE} non trovato.")
        return

    with open(INDEX_FILE, 'r') as f:
        config = yaml.safe_load(f)

    print("🧠 TazLab Memory Awaken Protocol...")
    print("-" * 50)
    
    full_context = "# TAZLAB CURRENT OPERATIONAL CONTEXT\n\n"

    for step in config.get('boot_sequence', []):
        category = step.get('category')
        query = step.get('query')
        
        print(f"🔍 Recupero contesto: {category}...")
        
        try:
            # Eseguo lo script mnemosyne in modalità ask
            # Usiamo --use-cli per evitare dipendenza da API key per la ricerca se possibile
            # Nota: Mnemosyne Engine richiede comunque embedding per la ricerca.
            # Se la chiave manca, tornerà risultati casuali o vuoti.
            result = subprocess.check_output(
                ["python3", str(MNEMOSYNE_SCRIPT), "--use-cli", "--no-dedup", "ask", query],
                stderr=subprocess.STDOUT,
                universal_newlines=True
            )
            
            # Estraggo la parte rilevante
            full_context += f"## {category}\n{result.strip()}\n\n"
            
        except Exception as e:
            print(f"  ⚠️  Errore durante il recupero di {category}: {e}")

    # Salvo il contesto
    with open(OUTPUT_FILE, 'w') as f:
        f.write(full_context)
    
    print("-" * 50)
    print(f"✨ Risveglio completato! Contesto salvato in: {OUTPUT_FILE}")

if __name__ == "__main__":
    awaken()

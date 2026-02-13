import yaml
import subprocess
import os

# Configurazione
INDEX_FILE = "/workspace/tazlab-memory/INDEX.yaml"
ARCHIVIST_SCRIPT = "/workspace/tazlab-memory/tazlab_archivist.py"

def awaken():
    if not os.path.exists(INDEX_FILE):
        print("❌ Errore: INDEX.yaml non trovato.")
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
            # Eseguo lo script archivista in modalità ricerca
            result = subprocess.check_output(
                ["python3", ARCHIVIST_SCRIPT, "search", query],
                stderr=subprocess.STDOUT,
                universal_newlines=True
            )
            
            # Estraggo solo la parte dei ricordi (dopo l'intestazione dello script)
            parts = result.split("🧠 Ricordi Strategici:")
            clean_result = parts[1].strip() if len(parts) > 1 else result.strip()
            
            full_context += f"## {category}\n{clean_result}\n\n"
            
        except Exception as e:
            print(f"  ⚠️  Errore durante il recupero di {category}: {e}")

    # Salvo il contesto
    context_file = "/workspace/tazlab-memory/CURRENT_CONTEXT.md"
    with open(context_file, 'w') as f:
        f.write(full_context)
    
    print("-" * 50)
    print(f"✨ Risveglio completato! Contesto salvato in: {context_file}")

if __name__ == "__main__":
    awaken()
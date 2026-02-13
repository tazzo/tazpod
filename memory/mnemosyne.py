import os
import sys
import json
import re
import time
import requests
import yaml
import argparse
import subprocess
from datetime import datetime
from pathlib import Path
from google import genai
from google.genai import types
import psycopg2

# --- Configuration ---
DB_HOST = os.getenv("DB_HOST", "192.168.1.241")
DB_PORT = os.getenv("DB_PORT", "5432")
DB_NAME = "mnemosyne"
DB_USER = "mnemosyne"
DB_PASS = os.getenv("DB_PASS", "[PoQAO46*M9hqUCDpXbciDJ.") # Should be in vault eventually
EMBEDDING_MODEL = "gemini-embedding-001"
AI_MODEL = "gemini-flash-latest"
INDEX_PATH = os.path.join(os.path.dirname(__file__), "INDEX.yaml")

CHUNK_SIZE = 200000
OVERLAP = 10000
API_PACE = 12 

class MnemosyneEngine:
    def __init__(self, use_cli=False, api_key=None):
        self.use_cli = use_cli
        self.api_key = api_key or self._get_api_key()
        self.client = None
        if not self.use_cli:
            if not self.api_key:
                print("❌ Error: GEMINI_API_KEY not found. Use --api-key or ensure it's in the vault.")
                sys.exit(1)
            self.client = genai.Client(api_key=self.api_key)
        self.taxonomy = self._load_taxonomy()

    def _get_api_key(self):
        if os.getenv("GEMINI_API_KEY"): return os.getenv("GEMINI_API_KEY")
        vault_key = "/home/tazpod/secrets/gemini-api-key"
        if os.path.exists(vault_key):
            with open(vault_key, "r") as f: return f.read().strip()
        return None

    def _load_taxonomy(self):
        with open(INDEX_PATH, 'r') as f: return yaml.safe_load(f).get('taxonomy', {})

    def get_embedding(self, text):
        # Embeddings are still done via API for precision and vector size consistency
        url = f"https://generativelanguage.googleapis.com/v1beta/models/{EMBEDDING_MODEL}:embedContent?key={self.api_key}"
        payload = {"content": {"parts": [{"text": text}]}}
        for _ in range(3):
            try:
                time.sleep(2)
                res = requests.post(url, json=payload)
                if res.status_code == 200: return res.json()['embedding']['values']
                elif res.status_code == 429: time.sleep(30)
            except: pass
        return [0.0] * 3072

    def generate_facts(self, chunk):
        prompt = f"Sei Senior Platform Architect. Estrai fatti tecnici autoconsistenti. Rispondi SOLO con un JSON array di oggetti (context, tags, event). Tassonomia: {json.dumps(self.taxonomy)}

LOG:
{chunk}"
        
        if self.use_cli:
            return self._call_cli(prompt)
        else:
            return self._call_sdk(prompt)

    def _call_sdk(self, prompt):
        time.sleep(API_PACE)
        response = self.client.models.generate_content(
            model=AI_MODEL, 
            contents=[prompt],
            config=types.GenerateContentConfig(response_mime_type='application/json')
        )
        return json.loads(response.text)

    def _call_cli(self, prompt):
        # Wrapper for 'gemini' CLI command
        try:
            process = subprocess.Popen(['gemini', 'ask', '--json'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            stdout, stderr = process.communicate(input=prompt)
            if process.returncode != 0:
                print(f"    ⚠️ CLI Error: {stderr}")
                return []
            # Extract JSON from potential CLI markdown or text
            match = re.search(r'\[.*\]', stdout, re.DOTALL)
            if match:
                return json.loads(match.group(0))
            return json.loads(stdout)
        except Exception as e:
            print(f"    ⚠️ CLI Parsing Error: {e}")
            return []

    def get_enrichment_decision(self, new_fact, similar_facts):
        if not similar_facts: return True
        prompt = f"Sei Supervisore TazLab. Confronta il NUOVO RICORDO con quelli esistenti. SCARTA se il risultato tecnico è identico. SALVA se ci sono aggiornamenti tecnici o nuovi contesti. NUOVO: {new_fact}
ESISTENTI: {json.dumps(similar_facts)}
Rispondi solo SALVA o SCARTA."
        
        if self.use_cli:
            res = self._call_cli(prompt) # Should be a simple string check
            return "SALVA" in str(res).upper()
        else:
            response = self.client.models.generate_content(model=AI_MODEL, contents=[prompt])
            return "SALVA" in response.text.upper()

def get_db_connection():
    return psycopg2.connect(host=DB_HOST, port=DB_PORT, database=DB_NAME, user=DB_USER, password=DB_PASS, connect_timeout=10)

def search_similar(cursor, embedding, limit=3):
    vector_str = "[" + ",".join(map(str, embedding)) + "]"
    sql = "SELECT content FROM memories ORDER BY embedding <=> %s LIMIT %s"
    cursor.execute(sql, (vector_str, limit))
    return [row[0] for row in cursor.fetchall()]

def process_file(engine, conn, file_path):
    fname = os.path.basename(file_path)
    with open(file_path, 'r') as f: data = f.read()
    if not data.strip(): return
    
    # Date Recovery
    session_date = datetime.fromtimestamp(os.path.getmtime(file_path))
    date_match = re.search(r'(\d{4}-\d{2}-\d{2})', fname)
    if date_match:
        try: session_date = datetime.strptime(date_match.group(1), "%Y-%m-%d")
        except: pass

    print(f"
🧠 Processing: {fname} ({len(data)} chars)")
    cursor = conn.cursor()
    
    start = 0
    while start < len(data):
        chunk = data[start:start+CHUNK_SIZE]
        try:
            facts = engine.generate_facts(chunk)
            if not isinstance(facts, list): facts = [facts]
            
            for fact in facts:
                if not isinstance(fact, dict) or not fact.get('event'): continue
                unified = f"{fact.get('context')} | {', '.join(fact.get('tags', []))} | {fact.get('event')}"
                vector = engine.get_embedding(unified)
                similar = search_similar(cursor, vector)
                
                if engine.get_enrichment_decision(unified, similar):
                    cursor.execute("INSERT INTO memories (timestamp, content, embedding) VALUES (%s, %s, %s)", (session_date, unified, vector))
                    print(f"      ✅ SAVED: {fact.get('context')[:50]}...")
            conn.commit()
        except Exception as e:
            print(f"    ⚠️ Error in chunk: {e}")
        
        start += (CHUNK_SIZE - OVERLAP)
        if start >= len(data): break
    
    cursor.execute("INSERT INTO archived_files (filename) VALUES (%s) ON CONFLICT DO NOTHING", (fname,))
    conn.commit()
    cursor.close()

def main():
    parser = argparse.ArgumentParser(description="Mnemosyne Unified Ingestor")
    parser.add_argument("target", nargs="?", default=".", help="File or directory to process")
    parser.add_argument("--use-api", action="store_true", help="Use direct Google API (default)")
    parser.add_argument("--use-cli", action="store_true", help="Use 'gemini' CLI tool")
    parser.add_argument("--api-key", help="Gemini API Key")
    parser.add_argument("--force", action="store_true", help="Process even if already archived")
    args = parser.parse_args()

    engine = MnemosyneEngine(use_cli=args.use_cli, api_key=args.api_key)
    conn = get_db_connection()
    
    target = Path(args.target)
    files_to_process = []
    
    if target.is_file():
        files_to_process.append(target)
    else:
        print(f"🔍 Searching for JSON files in {target.absolute()}...")
        files_to_process = sorted(list(target.rglob("*.json")))

    print(f"🎯 Found {len(files_to_process)} candidate files.")

    for f in files_to_process:
        if not args.force:
            cursor = conn.cursor()
            cursor.execute("SELECT 1 FROM archived_files WHERE filename = %s", (f.name,))
            if cursor.fetchone():
                cursor.close()
                continue
            cursor.close()
        
        process_file(engine, conn, f)

    print("
🏁 Ingestion complete.")
    conn.close()

if __name__ == "__main__":
    main()

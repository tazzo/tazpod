import os
import sys
import json
import re
import time
import random
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
DB_PASS = os.getenv("DB_PASS", "dyUuu54TOA8zGMkc)4JFNLYF")
EMBEDDING_MODEL = "gemini-embedding-001"
AI_MODEL = "gemini-2.0-flash-001"

# Paths relative to script
BASE_DIR = Path(__file__).parent
EXTRACTION_BLUEPRINT = BASE_DIR / "extraction_blueprint.md"
INDEX_PATH = BASE_DIR / "INDEX.yaml"

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
                print("❌ Error: GEMINI_API_KEY not found. Pass it via --api-key or environment.", file=sys.stderr)
                sys.exit(1)
            self.client = genai.Client(api_key=self.api_key)
        
        self.extraction_prompt = self._load_blueprint(EXTRACTION_BLUEPRINT)

    def _get_api_key(self):
        if os.getenv("GEMINI_API_KEY"): return os.getenv("GEMINI_API_KEY")
        vault_key = "/home/tazpod/secrets/gemini-api-key"
        if os.path.exists(vault_key):
            try:
                with open(vault_key, "r") as f: return f.read().strip().strip("'\"")
            except: pass
        return None

    def _load_blueprint(self, path):
        if not os.path.exists(path): return "Estrai fatti tecnici High-Res. Rispondi in JSON array: [{\"context\": \"...\", \"tags\": [\"...\"], \"event\": \"...\"}]"
        with open(path, 'r') as f: return f.read()

    def get_embedding(self, text):
        if not self.api_key: return [0.0] * 3072
        url = f"https://generativelanguage.googleapis.com/v1beta/models/{EMBEDDING_MODEL}:embedContent?key={self.api_key}"
        payload = {"content": {"parts": [{"text": text}]}}
        for _ in range(3):
            try:
                time.sleep(2)
                res = requests.post(url, json=payload, timeout=10)
                if res.status_code == 200: return res.json()['embedding']['values']
                elif res.status_code == 429: time.sleep(30)
            except: pass
        return [0.0] * 3072

    def extract_facts(self, log_content):
        prompt = f"{self.extraction_prompt}\n\n--- SESSION LOG BELOW ---\n{log_content}"
        if self.use_cli:
            return self._call_cli(prompt, is_json=True)
        else:
            return self._call_sdk(prompt, is_json=True)

    def _call_sdk(self, prompt, is_json=False):
        max_retries = 3
        for attempt in range(max_retries):
            try:
                time.sleep(API_PACE)
                config = None
                if is_json:
                    config = types.GenerateContentConfig(response_mime_type='application/json')
                
                response = self.client.models.generate_content(
                    model=AI_MODEL, 
                    contents=[prompt],
                    config=config
                )
                if is_json:
                    try: return json.loads(response.text)
                    except: return []
                return response.text
            except Exception as e:
                if attempt < max_retries - 1 and ("503" in str(e) or "UNAVAILABLE" in str(e)):
                    print(f"    ⏳ SDK 503. Waiting 60s (Attempt {attempt+1}/{max_retries})...", file=sys.stderr)
                    time.sleep(60)
                    continue
                raise e
        return [] if is_json else ""

    def _call_cli(self, prompt, is_json=False):
        stdout = ""
        max_retries = 3
        for attempt in range(max_retries):
            try:
                cmd = ['gemini']
                if is_json: cmd.extend(['-o', 'json'])
                
                process = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                stdout, stderr = process.communicate(input=prompt)
                
                # Filtro warning per evitare falsi positivi
                clean_stderr = "\n".join([line for line in stderr.splitlines() 
                                        if "DeprecationWarning" not in line and 
                                        "Loaded cached credentials" not in line and
                                        "Loading extension" not in line and
                                        "Hook registry" not in line])

                if process.returncode != 0:
                    if "503" in stderr or "UNAVAILABLE" in stderr:
                        print(f"    ⏳ Gemini busy (503). Waiting 60s...", file=sys.stderr)
                        time.sleep(60)
                        continue
                    if clean_stderr.strip():
                        print(f"    ⚠️ CLI Error: {clean_stderr}", file=sys.stderr)
                        raise Exception(f"CLI Error: {clean_stderr}")

                if is_json:
                    # Rimuoviamo eventuali righe di warning da stdout prima del parse
                    lines = stdout.splitlines()
                    json_lines = [l for l in lines if not l.startswith("(") and "Loaded cached credentials" not in l]
                    clean_stdout = "\n".join(json_lines)
                    try:
                        cli_data = json.loads(clean_stdout)
                        raw_res = cli_data.get('response', '')
                        clean_res = re.sub(r'```json\s*|\s*```', '', raw_res).strip()
                        return json.loads(clean_res)
                    except:
                        return []
                return stdout
            except Exception as e:
                if attempt < max_retries - 1 and ("503" in str(e) or "UNAVAILABLE" in str(e)):
                    time.sleep(60)
                    continue
                print(f"    ⚠️ CLI Exception: {e}", file=sys.stderr)
                raise e
        return [] if is_json else ""

    def get_enrichment_decision(self, new_fact, similar_facts):
        return True 

def get_db_connection():
    return psycopg2.connect(host=DB_HOST, port=DB_PORT, database=DB_NAME, user=DB_USER, password=DB_PASS, connect_timeout=10)

def recover_date(file_path, extracted_facts=None):
    fname = os.path.basename(file_path)
    date_match = re.search(r'(\d{4}-\d{2}-\d{2})', fname)
    if date_match:
        try: return datetime.strptime(date_match.group(1), "%Y-%m-%d")
        except: pass
    if extracted_facts and len(extracted_facts) > 0:
        ts = extracted_facts[0].get('ts')
        if ts:
            try: return datetime.fromisoformat(ts.replace('Z', '+00:00'))
            except: pass
    return datetime.fromtimestamp(os.path.getmtime(file_path))

def format_memory(fact):
    tags_str = ", ".join(fact.get('tags', []))
    return f"[{fact.get('context', 'No Context')}] | [{tags_str}] | [{fact.get('event', 'No Event')}]"

def cmd_extract(engine, file_path):
    print(f"🧪 Extracting facts from {file_path} to JSON...", file=sys.stderr)
    with open(file_path, 'r', encoding='utf-8') as f: content = f.read()
    facts = []
    start = 0
    total_len = len(content)
    while start < total_len:
        chunk = content[start:start+CHUNK_SIZE]
        chunk_facts = engine.extract_facts(chunk)
        if isinstance(chunk_facts, list): facts.extend(chunk_facts)
        start += (CHUNK_SIZE - OVERLAP)
        if start >= total_len: break
    
    out_file = Path(file_path).with_suffix(".json")
    with open(out_file, 'w') as f: json.dump(facts, f, indent=2)
    print(f"✅ Extracted {len(facts)} facts to {out_file}", file=sys.stderr)

def cmd_load(engine, json_path):
    print(f"📥 Loading facts from {json_path} into DB...", file=sys.stderr)
    with open(json_path, 'r') as f: facts = json.load(f)
    conn = get_db_connection()
    cursor = conn.cursor()
    file_date = recover_date(json_path, facts)
    
    for fact in facts:
        unified = format_memory(fact)
        vector = engine.get_embedding(unified)
        cursor.execute("INSERT INTO memories (timestamp, content, embedding) VALUES (%s, %s, %s::vector)", (file_date, unified, vector))
    
    conn.commit()
    conn.close()
    print(f"✅ Loaded {len(facts)} memories.", file=sys.stderr)

def cmd_sync(engine, target_dir, limit_files=None):
    target = Path(target_dir)
    files = sorted(list(target.rglob("*.md")))
    conn = get_db_connection()
    total_files = len(files)
    print(f"🔄 Syncing directory: {target.absolute()} ({total_files} files found)", file=sys.stderr)
    
    processed = 0
    for i, f in enumerate(files, 1):
        if limit_files and processed >= limit_files: break
        
        cursor = conn.cursor()
        cursor.execute("SELECT 1 FROM archived_files WHERE filename = %s", (f.name,))
        if cursor.fetchone():
            cursor.close()
            continue
        cursor.close()
        
        if i > 1:
            wait = random.randint(5, 15)
            print(f"\n⏳ Cooling down for {wait}s before next file...", file=sys.stderr)
            time.sleep(wait)

        with open(f, 'r', encoding='utf-8') as fh: content = fh.read()
        total_len = len(content)
        chunk_count = (total_len // (CHUNK_SIZE - OVERLAP)) + 1
        
        print(f"\n🧠 [{i}/{total_files}] Processing: {f.name}", file=sys.stderr)
        print(f"   📊 Size: {total_len} chars | Chunks: {chunk_count}", file=sys.stderr)
        
        try:
            facts = []
            start, current_chunk = 0, 1
            while start < total_len:
                print(f"   -> Analyzing Chunk {current_chunk}/{chunk_count}... ", end="", file=sys.stderr, flush=True)
                chunk_facts = engine.extract_facts(content[start:start+CHUNK_SIZE])
                if isinstance(chunk_facts, list):
                    facts.extend(chunk_facts)
                    print(f"OK ({len(chunk_facts)} facts)", file=sys.stderr)
                else:
                    print("FAILED", file=sys.stderr)
                start += (CHUNK_SIZE - OVERLAP)
                current_chunk += 1
                if start >= total_len: break
                
            file_date = recover_date(f, facts)
            print(f"   📅 Target Date: {file_date.strftime('%Y-%m-%d')}", file=sys.stderr)
            
            print(f"   📥 Saving {len(facts)} memories... ", file=sys.stderr, flush=True)
            saved = 0
            cursor = conn.cursor()
            for j, fact in enumerate(facts, 1):
                unified = format_memory(fact)
                vector = engine.get_embedding(unified)
                cursor.execute("INSERT INTO memories (timestamp, content, embedding) VALUES (%s, %s, %s::vector)", (file_date, unified, vector))
                saved += 1
                if j % 5 == 0 or j == len(facts):
                    print(f"{j}..", end="", file=sys.stderr, flush=True)
            
            cursor.execute("INSERT INTO archived_files (filename) VALUES (%s)", (f.name,))
            conn.commit()
            print(f" DONE (Saved: {saved})", file=sys.stderr)
            cursor.close()
            processed += 1
        except Exception as e:
            conn.rollback()
            print(f"🛑 Error processing {f.name}: {e}. Stopping.", file=sys.stderr)
            conn.close()
            sys.exit(1)
            
    conn.close()

def cmd_wipe():
    conn = get_db_connection()
    cursor = conn.cursor()
    cursor.execute("DELETE FROM memories; DELETE FROM archived_files;")
    conn.commit()
    conn.close()
    print("💥 Memory wiped (DELETE).", file=sys.stderr)

def main():
    parser = argparse.ArgumentParser(description="Mnemosyne: Semantic Memory Engine")
    parser.add_argument("--use-cli", action="store_true", help="Use 'gemini' CLI")
    parser.add_argument("--api-key", help="Gemini API Key")
    parser.add_argument("--no-dedup", action="store_true", help="Disable AI deduplication")
    
    subparsers = parser.add_subparsers(dest="command")
    subparsers.add_parser("extract").add_argument("file")
    subparsers.add_parser("load").add_argument("file")
    subparsers.add_parser("sync").add_argument("dir", nargs="?", default=".")
    subparsers.add_parser("next").add_argument("dir")
    subparsers.add_parser("wipe")
    
    args = parser.parse_args()
    if not args.command: return
    if args.command == "wipe": cmd_wipe(); return

    engine = MnemosyneEngine(use_cli=args.use_cli, api_key=args.api_key)
    if args.command == "extract": cmd_extract(engine, args.file)
    elif args.command == "load": cmd_load(engine, args.file)
    elif args.command == "sync": cmd_sync(engine, args.dir)
    elif args.command == "next": cmd_sync(engine, args.dir, limit_files=1)

if __name__ == "__main__":
    main()

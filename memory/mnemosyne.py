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
DB_NAME = os.getenv("DB_NAME", "mnemosyne")
DB_USER = os.getenv("DB_USER", "mnemosyne")
DB_PASS = os.getenv("DB_PASS", "[PoQAO46*M9hqUCDpXbciDJ.")
EMBEDDING_MODEL = "gemini-embedding-001"
AI_MODEL = "gemini-flash-latest"

# Paths relative to script
BASE_DIR = Path(__file__).parent
EXTRACTION_BLUEPRINT = BASE_DIR / "extraction_blueprint.md"
DEDUPLICATION_BLUEPRINT = BASE_DIR / "deduplication_blueprint.md"
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
                print("❌ Error: GEMINI_API_KEY not found. Pass it via --api-key or environment.")
                sys.exit(1)
            self.client = genai.Client(api_key=self.api_key)
        
        self.extraction_prompt = self._load_blueprint(EXTRACTION_BLUEPRINT)
        self.dedup_prompt_tmpl = self._load_blueprint(DEDUPLICATION_BLUEPRINT)

    def _get_api_key(self):
        if os.getenv("GEMINI_API_KEY"): return os.getenv("GEMINI_API_KEY")
        vault_key = "/home/tazpod/secrets/gemini-api-key"
        if os.path.exists(vault_key):
            with open(vault_key, "r") as f: return f.read().strip().strip("'\"")
        return None

    def _load_blueprint(self, path):
        with open(path, 'r') as f: return f.read()

    def get_embedding(self, text):
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

    def extract_facts(self, log_content):
        # High-Resolution Extraction using Blueprint
        prompt = f"{self.extraction_prompt}\n\n--- SESSION LOG BELOW ---\n{log_content}"
        
        if self.use_cli:
            return self._call_cli(prompt, is_json=True)
        else:
            return self._call_sdk(prompt, is_json=True)

    def _call_sdk(self, prompt, is_json=False):
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

    def _call_cli(self, prompt, is_json=False):
        try:
            cmd = ['gemini', 'ask']
            if is_json: cmd.append('--json')
            process = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            stdout, stderr = process.communicate(input=prompt)
            if process.returncode != 0:
                print(f"    ⚠️ CLI Error: {stderr}")
                return [] if is_json else ""
            
            if is_json:
                match = re.search(r'\[.*\]', stdout, re.DOTALL)
                if match: return json.loads(match.group(0))
                return json.loads(stdout)
            return stdout
        except Exception as e:
            print(f"    ⚠️ CLI Exception: {e}")
            return [] if is_json else ""

    def get_enrichment_decision(self, new_fact, similar_facts):
        if not similar_facts: return True
        
        prompt = self.dedup_prompt_tmpl.replace("{{NEW_FACT}}", new_fact)
        prompt = prompt.replace("{{EXISTING_SIMILAR_MEMORIES}}", json.dumps(similar_facts))
        
        res = ""
        if self.use_cli:
            res = self._call_cli(prompt)
        else:
            res = self._call_sdk(prompt)
        
        return "SAVE" in res.upper()

def get_db_connection():
    return psycopg2.connect(host=DB_HOST, port=DB_PORT, database=DB_NAME, user=DB_USER, password=DB_PASS, connect_timeout=10)

def recover_date(file_path, extracted_facts):
    # Hierarchy: 1. Filename, 2. Facts TS, 3. File MTime
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

def search_similar(cursor, embedding, limit=3, days=None):
    vector_str = "[" + ",".join(map(str, embedding)) + "]"
    sql = "SELECT content FROM memories"
    params = [vector_str]
    
    if days:
        sql += " WHERE timestamp > NOW() - INTERVAL '%s days'"
        params.append(str(days))
    
    sql += " ORDER BY embedding <=> %s LIMIT %s"
    params.append(limit)
    
    cursor.execute(sql, tuple(params))
    return [row[0] for row in cursor.fetchall()]

def format_memory(fact):
    # Header: [CONTEXT] | [TAGS]
    # Body: Event
    tags_str = ", ".join(fact.get('tags', []))
    header = f"[{fact.get('context')}] | [{tags_str}]"
    return f"{header}\n{fact.get('event')}"

def cmd_extract(engine, target_path):
    target = Path(target_path)
    if not target.is_file():
        print(f"❌ Error: {target_path} is not a file.")
        return
    
    with open(target, 'r') as f: content = f.read()
    print(f"🔍 Extracting facts from {target.name}...")
    
    facts = []
    # Handle chunking for extraction
    start = 0
    while start < len(content):
        chunk = content[start:start+CHUNK_SIZE]
        chunk_facts = engine.extract_facts(chunk)
        if isinstance(chunk_facts, list): facts.extend(chunk_facts)
        start += (CHUNK_SIZE - OVERLAP)
        if start >= len(content): break
        
    print(json.dumps(facts, indent=2))

def cmd_load(engine, facts_json_path):
    with open(facts_json_path, 'r') as f: facts = json.load(f)
    conn = get_db_connection()
    cursor = conn.cursor()
    
    print(f"📥 Loading {len(facts)} facts into database...")
    for fact in facts:
        unified = format_memory(fact)
        ts = fact.get('ts', datetime.now().isoformat())
        vector = engine.get_embedding(unified)
        similar = search_similar(cursor, vector)
        
        if engine.get_enrichment_decision(unified, similar):
            cursor.execute("INSERT INTO memories (timestamp, content, embedding) VALUES (%s, %s, %s)", (ts, unified, vector))
            print(f"  ✅ Saved: {fact.get('context')[:50]}...")
    
    conn.commit()
    conn.close()

def cmd_sync(engine, target_dir):
    target = Path(target_dir)
    files = sorted(list(target.rglob("*.json")))
    conn = get_db_connection()
    
    print(f"🔄 Syncing directory: {target.absolute()}")
    for f in files:
        cursor = conn.cursor()
        cursor.execute("SELECT 1 FROM archived_files WHERE filename = %s", (f.name,))
        if cursor.fetchone():
            cursor.close()
            continue
        cursor.close()
        
        # Process full cycle
        with open(f, 'r') as file_handle: content = file_handle.read()
        print(f"\n🧠 Processing {f.name}...")
        
        facts = []
        start = 0
        while start < len(content):
            chunk = content[start:start+CHUNK_SIZE]
            chunk_facts = engine.extract_facts(chunk)
            if isinstance(chunk_facts, list): facts.extend(chunk_facts)
            start += (CHUNK_SIZE - OVERLAP)
            if start >= len(content): break
            
        file_date = recover_date(f, facts)
        
        cursor = conn.cursor()
        for fact in facts:
            unified = format_memory(fact)
            vector = engine.get_embedding(unified)
            similar = search_similar(cursor, vector)
            if engine.get_enrichment_decision(unified, similar):
                cursor.execute("INSERT INTO memories (timestamp, content, embedding) VALUES (%s, %s, %s)", (file_date, unified, vector))
                print(f"  ✅ Saved Fact from {file_date.strftime('%Y-%m-%d')}")
        
        cursor.execute("INSERT INTO archived_files (filename) VALUES (%s)", (f.name,))
        conn.commit()
        
    conn.close()

def cmd_ask(engine, query, days=None):
    vector = engine.get_embedding(query)
    conn = get_db_connection()
    cursor = conn.cursor()
    
    sql = "SELECT timestamp, content FROM memories"
    params = [vector]
    if days:
        sql += " WHERE timestamp > NOW() - INTERVAL '%s days'"
        params.append(str(days))
    
    sql += " ORDER BY embedding <=> %s LIMIT 5"
    cursor.execute(sql, tuple(params))
    rows = cursor.fetchall()
    
    print(f"\n🔎 Top 5 memories for: '{query}'")
    if days: print(f"📅 Filtered to last {days} days.")
    print("-" * 40)
    for row in rows:
        print(f"\n[DATE: {row[0].strftime('%Y-%m-%d')}]")
        print(row[1])
    
    conn.close()

def main():
    parser = argparse.ArgumentParser(description="Mnemosyne: Semantic Memory Engine")
    parser.add_argument("--use-cli", action="store_true", help="Use 'gemini' CLI instead of SDK")
    parser.add_argument("--api-key", help="Gemini API Key")
    
    subparsers = parser.add_subparsers(dest="command", help="Subcommand to run")
    
    # Extract
    p_extract = subparsers.add_parser("extract", help="Extract facts from a log file to JSON")
    p_extract.add_argument("file", help="Source JSON log file")
    
    # Load
    p_load = subparsers.add_parser("load", help="Load extracted facts from JSON to DB")
    p_load.add_argument("file", help="Source facts JSON file")
    
    # Sync
    p_sync = subparsers.add_parser("sync", help="Full automated sync of a directory")
    p_sync.add_argument("dir", nargs="?", default=".", help="Directory to sync (default: current)")
    
    # Ask
    p_ask = subparsers.add_parser("ask", help="Search semantic memory")
    p_ask.add_argument("query", help="Search query")
    p_ask.add_argument("-d", "--days", type=int, help="Limit results to last N days")
    
    # Delete
    p_delete = subparsers.add_parser("delete", help="Delete a memory by ID (Not yet implemented via CLI)")
    
    args = parser.parse_args()
    if not args.command:
        parser.print_help()
        return

    engine = MnemosyneEngine(use_cli=args.use_cli, api_key=args.api_key)
    
    if args.command == "extract": cmd_extract(engine, args.file)
    elif args.command == "load": cmd_load(engine, args.file)
    elif args.command == "sync": cmd_sync(engine, args.dir)
    elif args.command == "ask": cmd_ask(engine, args.query, args.days)

if __name__ == "__main__":
    main()

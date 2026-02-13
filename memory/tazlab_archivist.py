import os
import json
import psycopg2
import re
import time
import requests
import yaml
from datetime import datetime
from google import genai
from google.genai import types

# --- Configuration ---
DB_HOST = os.getenv("DB_HOST", "192.168.1.241")
DB_PORT = os.getenv("DB_PORT", "5432")
DB_NAME = "mnemosyne"
DB_USER = "mnemosyne"
DB_PASS = "[PoQAO46*M9hqUCDpXbciDJ."
EMBEDDING_MODEL = "gemini-embedding-001"
AI_MODEL = "gemini-flash-latest" # High-Res and Free Tier confirmed
INDEX_PATH = os.path.join(os.path.dirname(__file__), "INDEX.yaml")

CHUNK_SIZE = 200000
OVERLAP = 10000
API_PACE = 12 # Higher delay to be extra safe

def get_api_key():
    try:
        if os.getenv("GEMINI_API_KEY"): return os.getenv("GEMINI_API_KEY")
        with open("/home/tazpod/secrets/gemini-api-key", "r") as f: return f.read().strip()
    except: return None

def get_db_connection():
    return psycopg2.connect(host=DB_HOST, port=DB_PORT, database=DB_NAME, user=DB_USER, password=DB_PASS, connect_timeout=15)

def call_ai_with_retry(client, model, contents, config=None, retries=3):
    for i in range(retries):
        try:
            print(f"    📡 Calling Google API ({model})...")
            time.sleep(API_PACE)
            if config:
                return client.models.generate_content(model=model, contents=contents, config=config)
            return client.models.generate_content(model=model, contents=contents)
        except Exception as e:
            if "429" in str(e) or "RESOURCE_EXHAUSTED" in str(e):
                wait = (i + 1) * 60 # Strict backoff: 60s, 120s...
                print(f"    ⏳ Rate limit. Waiting {wait}s...")
                time.sleep(wait)
            else: raise e
    raise Exception("Max retries exceeded")

def get_embedding(text):
    api_key = get_api_key()
    url = f"https://generativelanguage.googleapis.com/v1beta/models/{EMBEDDING_MODEL}:embedContent?key={api_key}"
    payload = {"content": {"parts": [{"text": text}]}}
    for i in range(3):
        try:
            time.sleep(API_PACE)
            res = requests.post(url, json=payload)
            if res.status_code == 200: return res.json()['embedding']['values']
            elif res.status_code == 429:
                time.sleep(60)
            else: raise Exception(f"Embedding Error: {res.text}")
        except: pass
    return [0.0] * 3072 # Fallback to empty vector if embedding fails

def parse_session_date(log_data, log_file):
    file_match = re.search(r'(\d{4}-\d{2}-\d{2})', os.path.basename(log_file))
    if file_match:
        try: return datetime.strptime(file_match.group(1), "%Y-%m-%d")
        except: pass
    return datetime.fromtimestamp(os.path.getmtime(log_file))

def load_taxonomy():
    with open(INDEX_PATH, 'r') as f: return yaml.safe_load(f).get('taxonomy', {})

def get_enrichment_decision(client, new_fact, similar_facts):
    if not similar_facts: return True
    prompt = f"Sei Supervisore TazLab. Confronta il NUOVO RICORDO con quelli esistenti. SCARTA se il risultato tecnico è identico. SALVA se ci sono aggiornamenti tecnici, numerici o di stato. NUOVO: {new_fact}\nESISTENTI: {json.dumps(similar_facts)}\nRispondi solo SALVA o SCARTA."
    try:
        response = call_ai_with_retry(client, AI_MODEL, [prompt])
        return "SALVA" in response.text.upper()
    except: return True

def search_similar(cursor, embedding, limit=3):
    vector_str = "[" + ",".join(map(str, embedding)) + "]"
    sql = "SELECT content FROM memories ORDER BY embedding <=> %s LIMIT %s"
    cursor.execute(sql, (vector_str, limit))
    return [row[0] for row in cursor.fetchall()]

def process_file(conn, log_file):
    fname = os.path.basename(log_file)
    with open(log_file, 'r') as f: log_data = f.read()
    if not log_data.strip(): return False
    session_date = parse_session_date(log_data, log_file)
    print(f"\n🚀 [DATE: {session_date.strftime('%Y-%m-%d')}] Ingesting: {fname}")
    
    client = genai.Client(api_key=get_api_key())
    taxonomy = load_taxonomy()
    start = 0
    total_len = len(log_data)
    cursor = conn.cursor()
    
    while start < total_len:
        chunk = log_data[start:start+CHUNK_SIZE]
        prompt = f"Sei Senior Platform Architect. Estrai cronache tecniche High-Res. JSON array di oggetti (context, tags, event). Tassonomia: {json.dumps(taxonomy)}"
        try:
            response = call_ai_with_retry(client, AI_MODEL, [prompt, f"LOG:\n{chunk}"], config=types.GenerateContentConfig(response_mime_type='application/json'))
            facts = json.loads(response.text)
            if not isinstance(facts, list): facts = [facts]
            for fact in facts:
                if not isinstance(fact, dict) or not fact.get('event'): continue
                unified = f"{fact.get('context')} | {', '.join(fact.get('tags', []))} | {fact.get('event')}"
                vector = get_embedding(unified)
                similar = search_similar(cursor, vector)
                if get_enrichment_decision(client, unified, similar):
                    cursor.execute("INSERT INTO memories (timestamp, content, embedding) VALUES (%s, %s, %s)", (session_date, unified, vector))
                    print(f"      ✅ SAVED: {fact.get('context')[:50]}...")
            conn.commit()
        except Exception as e: print(f"    ⚠️ Error: {e}")
        start += (CHUNK_SIZE - OVERLAP)
        if start >= total_len: break
    
    cursor.execute("INSERT INTO archived_files (filename) VALUES (%s) ON CONFLICT DO NOTHING", (fname,))
    conn.commit()
    cursor.close()
    return True

def archive_one_file(dir_path):
    print("🎯 Searching for the next file to archive...")
    conn = get_db_connection()
    files = sorted([f for f in os.listdir(dir_path) if f.endswith('.json')])
    for f in files:
        cur = conn.cursor()
        cur.execute("SELECT 1 FROM archived_files WHERE filename = %s", (f,))
        if cur.fetchone():
            cur.close()
            continue
        cur.close()
        if process_file(conn, os.path.join(dir_path, f)):
            print(f"✅ Successfully archived: {f}")
            conn.close()
            return # Exit after one file
    print("🏁 No new files to archive.")
    conn.close()

if __name__ == "__main__":
    import sys
    if len(sys.argv) > 2 and sys.argv[1] == "batch": archive_one_file(sys.argv[2])
    elif len(sys.argv) > 2 and sys.argv[1] == "single":
        c = get_db_connection(); process_file(c, sys.argv[2]); c.close()
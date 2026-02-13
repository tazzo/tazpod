import json
import sys
import os
import time
sys.path.append('/workspace/tazlab-memory')
from tazlab_archivist import get_embedding, get_db_connection, search_similar, call_ai_with_retry, genai

INPUT_FILE = "/workspace/tazlab-memory/extraction_results.json"
DEDUP_BLUEPRINT = "/workspace/conductor/deduplication_blueprint.md"

def get_blueprint_decision(client, new_fact, similar_facts):
    if not similar_facts: return True
    with open(DEDUP_BLUEPRINT, 'r') as f: blueprint = f.read()
    prompt = blueprint.replace("{{NEW_FACT}}", new_fact).replace("{{SIMILAR_FACTS}}", json.dumps(similar_facts, indent=2))
    try:
        response = call_ai_with_retry(client, "gemini-flash-latest", [prompt])
        return "SALVA" in response.text.upper()
    except: return True

def ingest(filename):
    if not os.path.exists(INPUT_FILE): return
    with open(INPUT_FILE, 'r') as f: facts = json.load(f)
    total = len(facts)
    print(f"\n🚀 MNEMOSYNE INGESTION: {filename} ({total} facts)")
    
    conn = get_db_connection()
    client = genai.Client(api_key=os.getenv("GEMINI_API_KEY") or open("/home/tazpod/secrets/gemini-api-key").read().strip())

    for i, fact in enumerate(facts, 1):
        if not isinstance(fact, dict) or not fact.get('event'): continue
        ts, ctx, tags, event = fact.get('ts', 'N/A'), fact.get('context', 'N/A'), fact.get('tags', []), fact.get('event', '')
        unified = f"{ctx} | {', '.join(tags)} | {event}"
        print(f"🔍 [{i}/{total}] {ctx[:50]}...")
        
        try:
            cursor = conn.cursor()
            vector = get_embedding(unified)
            similar = search_similar(cursor, vector)
            if get_blueprint_decision(client, unified, similar):
                cursor.execute("INSERT INTO memories (timestamp, content, embedding) VALUES (%s, %s, %s)", (ts if ts != 'N/A' else None, unified, vector))
                conn.commit() # ATOMIC COMMIT
                print("   ✅ SAVED")
            else: print("   ⏭️ SKIPPED")
            cursor.close()
            time.sleep(10) # Pacing
        except Exception as e: print(f"   ⚠️ Error: {e}")

    # Mark file as archived
    cur = conn.cursor()
    cur.execute("INSERT INTO archived_files (filename) VALUES (%s) ON CONFLICT DO NOTHING", (filename,))
    conn.commit()
    cur.close()
    conn.close()
    print("🏁 Done.")

if __name__ == "__main__":
    if len(sys.argv) > 1: ingest(sys.argv[1])

import psycopg2
try:
    conn = psycopg2.connect(host='34.154.223.12', port=5432, database='postgres', user='postgres', password='TazLabMemory2026!', sslmode='require')
    cur = conn.cursor()
    cur.execute('SELECT COUNT(*) FROM tazlab_knowledge;')
    count = cur.fetchone()[0]
    print(f'📊 Memories in AlloyDB: {count}')
    conn.close()
except Exception as e:
    print(f"❌ Error: {e}")

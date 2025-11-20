import sqlite3

conn = sqlite3.connect('iperf_tracker.db')
cursor = conn.cursor()

# Check traces
cursor.execute('SELECT id, destination_host, created_at, total_hops, completed FROM traces ORDER BY created_at DESC LIMIT 10')
traces = cursor.fetchall()

print(f'\n=== TRACES ({len(traces)} found) ===')
for row in traces:
    print(f'ID: {row[0]}, Host: {row[1]}, Created: {row[2]}, Hops: {row[3]}, Completed: {row[4]}')

# Check hops for latest trace
if traces:
    latest_trace_id = traces[0][0]
    cursor.execute('SELECT COUNT(*) FROM trace_hops WHERE trace_id = ?', (latest_trace_id,))
    hop_count = cursor.fetchone()[0]
    print(f'\n=== HOPS for latest trace (ID {latest_trace_id}) ===')
    print(f'Total hops in DB: {hop_count}')
    
    cursor.execute('SELECT hop_number, ip_address, hostname, city, country FROM trace_hops WHERE trace_id = ? ORDER BY hop_number LIMIT 5', (latest_trace_id,))
    hops = cursor.fetchall()
    for hop in hops:
        print(f'  Hop {hop[0]}: {hop[1]} ({hop[2]}) - {hop[3]}, {hop[4]}')

conn.close()

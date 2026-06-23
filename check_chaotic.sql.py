import sqlite3
conn = sqlite3.connect(r"C:\Users\undercurrent\.cache\codebase-memory-mcp\C-Pro2026-re0.db")
for row in conn.execute("SELECT name, file_path FROM nodes WHERE name LIKE '%ChaoticState%' LIMIT 5"):
    print(f"{row[0]} -> {row[1]}")
conn.close()

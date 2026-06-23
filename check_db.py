import sqlite3

db_path = r"C:\Users\undercurrent\.cache\codebase-memory-mcp\C-Pro2026-re0.db"
conn = sqlite3.connect(db_path)

print("=== 数据库中的表 ===")
for row in conn.execute("SELECT name FROM sqlite_master WHERE type='table'"):
    print(row[0])

print("\n=== nodes 表的列 ===")
for row in conn.execute("PRAGMA table_info(nodes)"):
    print(row)

print("\n=== 测试查询 prism_insert ===")
for row in conn.execute("SELECT name, file_path FROM nodes WHERE name LIKE '%prism_insert%' LIMIT 5"):
    print(f"{row[0]} -> {row[1]}")

conn.close()

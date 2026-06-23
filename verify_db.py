import sqlite3
conn = sqlite3.connect(r"C:\Users\undercurrent\.cache\codebase-memory-mcp\C-Pro2026-re0.db")
print("数据库连接成功！")
cursor = conn.execute("SELECT count(*) FROM nodes")
print(f"nodes表共有 {cursor.fetchone()[0]} 行数据。")
print("\n查找 'prism_insert' 的结果：")
for row in conn.execute("SELECT name, file_path FROM nodes WHERE name = 'prism_insert'"):
    print(f"  {row[0]} -> {row[1]}")
print("\n查找 'ChaoticState' 的结果：")
for row in conn.execute("SELECT name, file_path FROM nodes WHERE name = 'ChaoticState'"):
    print(f"  {row[0]} -> {row[1]}")
conn.close()

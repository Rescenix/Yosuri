import sqlite3
test_symbol = "prism_insert"  # 这是我们怀疑的，杉汐应该会传入的符号
conn = sqlite3.connect(r"C:\Users\undercurrent\.cache\codebase-memory-mcp\C-Pro2026-re0.db")
# 这里完全模拟Go代码里的SQL查询 'SELECT name, file_path FROM nodes WHERE name = ? LIMIT 10'
cursor = conn.execute("SELECT name, file_path FROM nodes WHERE name = ? LIMIT 10", (test_symbol,))
results = cursor.fetchall()
if results:
    print(f"✅ 使用符号 '{test_symbol}' 查询成功！")
    for row in results:
        print(f"  {row[0]} -> {row[1]}")
else:
    print(f"❌ 使用符号 '{test_symbol}' 未找到匹配项。可能是杉汐没有传入正确的符号名。")
conn.close()

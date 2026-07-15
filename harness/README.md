# Aurora Harness

长驻 Agent 运行时（Python/FastAPI，端口 **8001**）。re0 仓库的独立常驻后端子模块，
由 Go 后端（main-backend, :8080）经 HTTP 调用。对标 Hermes 的"独立常驻运行时"模型。

## 架构

```
nssm (Windows 服务, SERVICE_AUTO_START)
 └─ watchdog.py          进程守护：子进程崩溃后 2s 内自动重启，写 logs/watchdog.log
     └─ app.py           FastAPI :8001
         ├─ mcp_client.py     常驻 MCP stdio 客户端（newline-delimited JSON-RPC，
         │                    与 main-backend/internal/handler/mcp_client.go 同构）
         └─ log_sentinel.py   日志自检：tail harness.log，ERROR → incidents.log + 预设动作
```

## 端点

| 端点 | 说明 |
|---|---|
| `GET /health` | 存活探针 `{"status":"ok"}` |
| `GET /status` | uptime / 任务计数 / MCP server 状态 / 事件计数 / 日志路径 |
| `GET /run_task?tool=X&args={json}` | 内建任务（`count_lines`）或任意 MCP 工具 |
| `POST /debug/inject_error` | 注入合成 ERROR，验证日志自检链路 |

示例：

```bash
curl "http://localhost:8001/run_task?tool=list_directory&args=%7B%22path%22%3A%22C%3A%5C%5CPro2026%5C%5Cre0%22%7D"
curl "http://localhost:8001/run_task?tool=count_lines&args=%7B%22path%22%3A%22C%3A%5C%5CPro2026%5C%5Cre0%5C%5CREADME.md%22%7D"
```

## 运行

```bash
pip install -r requirements.txt
python watchdog.py        # 生产形态（带守护）
python app.py             # 裸跑（调试）
pytest tests/ -v          # 测试
```

## MCP 配置

`mcp.json`（与 Go 侧 `mcp.json` 格式一致），默认接 filesystem server（根 = C:\Pro2026\re0）：

```json
{"servers": {"fs": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "C:\\Pro2026\\re0"]}}}
```

工具既可用全名 `mcp__fs__list_directory`，也可用裸名 `list_directory`（无歧义时）。

## Windows 服务化

管理员运行 `install_nssm.bat`：注册 AuroraHarness（本 harness）与 AuroraBackend（Go server.exe），
均 SERVICE_AUTO_START。python 路径由 `where python` 动态解析（跳过 WindowsApps stub）。

## Go 侧调用

见 `main-backend/internal/handler/harness_client.go`：
`GET /api/harness/demo` → `http.Get("http://localhost:8001/run_task?...")`。

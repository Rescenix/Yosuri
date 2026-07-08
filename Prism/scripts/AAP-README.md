# AAP (Aurora Application Protocol) 客户端

MiMo Code 的 AAP 客户端适配器，用于替代 MCP 协议。

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                    AAP Server                            │
│         事件总线 · Agent注册 · 因果链审计                 │
├─────────────────────────────────────────────────────────┤
│  MiMo Code    │  Atri    │  其他Agent                    │
│  (AAP Client) │  (Agent) │  (Agent)                     │
└─────────────────────────────────────────────────────────┘
         ↕                   ↕
    PrismD (数字海马体)
```

## 快速开始

### 1. 启动 AAP 服务器

```bash
cd Prism/scripts
node aap-server.js --port 8081
```

### 2. 启动 MiMo Code AAP 客户端

```bash
node aap-client.js --name mimocode --aap-server http://localhost:8081 --port 5667
```

### 3. 测试通信

```bash
# 查看服务器状态
curl http://localhost:8081/status

# 发送测试事件
curl -X POST http://localhost:8081/event \
  -H "Content-Type: application/json" \
  -d '{"type":"AGENT_BROADCAST","from":"test","payload":{"message":"hello"}}'
```

## API

### AAP 事件类型

| 类型 | 说明 |
|------|------|
| `MEMORY_SYNC` | 记忆同步事件 |
| `AGENT_BROADCAST` | Agent 广播 |
| `AGENT_RESULT` | Agent 响应 |
| `CHAIN_HASH` | 因果链哈希 |
| `HEARTBEAT` | 心跳 |
| `AGENT_REGISTER` | Agent 注册 |

### AAPClient 方法

```javascript
const { AAPClient } = require('./aap-client.js');

const client = new AAPClient({
  name: 'mimocode',
  aapServer: 'http://localhost:8081',
  port: 5667
});

// 启动
await client.start();

// 广播事件
await client.broadcast('MEMORY_SYNC', {
  action: 'store',
  data: { role: 'user', content: '用户偏好记忆' }
});

// 监听事件
client.on('agent_broadcast', (event) => {
  console.log('收到广播:', event.payload);
});

// 关闭
await client.shutdown();
```

### 从 PrismD 召回记忆

```javascript
// 通过 AAP 发送召回请求
await client.broadcast('MEMORY_SYNC', {
  action: 'recall',
  data: { query: '用户偏好' }
});

// 监听结果
client.on('event', (event) => {
  if (event.type === 'AGENT_RESULT') {
    console.log('召回结果:', event.payload.memories);
  }
});
```

## 与 MCP 的对比

| 特性 | AAP | MCP |
|------|-----|-----|
| 通信模式 | 事件总线广播 | 请求-响应 |
| 记忆集成 | MemoryCoord 直接同步 | 无 |
| 审计追踪 | CausalChain 哈希链 | 无 |
| Agent 管理 | 注册/注销/心跳 | 无 |
| 协议开销 | 轻量级 | 较重 |

## 注意事项

1. AAP 服务器需要先启动
2. 客户端会自动重连（5秒间隔）
3. 心跳间隔 30 秒
4. 因果链提供事件审计追踪

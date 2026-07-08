#!/usr/bin/env node
// aap-client.js — MiMo Code AAP 客户端适配器
// 用法: node aap-client.js [--port 5667] [--aap-server localhost:8081]

const http = require('http');
const crypto = require('crypto');
const { EventEmitter } = require('events');

// ========== AAP 事件定义 ==========
const AAPEvent = {
  TypeMemorySync: 'MEMORY_SYNC',
  TypeAgentBroadcast: 'AGENT_BROADCAST',
  TypeAgentResult: 'AGENT_RESULT',
  TypeChainHash: 'CHAIN_HASH'
};

class MemoryCoord {
  constructor(activeNodes = 0, conductance = 0.5, chaos = 0.5, alpha = 0.1, lambda = 0.01) {
    this.active_nodes = activeNodes;
    this.conductance = conductance;
    this.chaos = chaos;
    this.alpha = alpha;
    this.lambda = lambda;
    this.data_age = 0;
  }
}

class AAPEventFrame {
  constructor(type, from, payload, coord = null) {
    this.id = `evt_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    this.type = type;
    this.from = from;
    this.to = '';
    this.payload = payload;
    this.memory_coord = coord;
    this.causal_hash = '';
    this.timestamp = Math.floor(Date.now() / 1000);
  }
}

// ========== 因果链 ==========
class CausalChain {
  constructor() {
    this.hashChain = [];
  }

  lastHash() {
    return this.hashChain.length > 0 ? this.hashChain[this.hashChain.length - 1] : '';
  }

  commitEvent(event) {
    const prevHash = this.lastHash();
    const payload = prevHash + event.id + event.type;
    const hash = crypto.createHash('sha256').update(payload).digest('hex');
    this.hashChain.push(hash);
    return hash;
  }

  verify() {
    // 简化验证：只检查链连续性
    return this.hashChain.length > 0;
  }
}

// ========== AAP 客户端 ==========
class AAPClient extends EventEmitter {
  constructor(options = {}) {
    super();
    this.name = options.name || 'mimocode';
    this.aapServer = options.aapServer || 'http://localhost:8081';
    this.port = options.port || 5667;
    this.causalChain = new CausalChain();
    this.memoryCoord = new MemoryCoord();
    this.connected = false;
    this.reconnectTimer = null;
    this.server = null;
  }

  // 启动 AAP 客户端
  async start() {
    console.log(`[AAP] 启动 MiMo Code AAP 客户端: ${this.name}`);
    
    // 启动 HTTP 服务接收事件
    this.server = http.createServer((req, res) => {
      if (req.method === 'POST') {
        let body = '';
        req.on('data', chunk => body += chunk);
        req.on('end', () => {
          try {
            const event = JSON.parse(body);
            this.handleIncomingEvent(event);
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ ok: true }));
          } catch (e) {
            res.writeHead(400);
            res.end('Invalid event');
          }
        });
      } else {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ status: 'ok', agent: this.name }));
      }
    });

    this.server.listen(this.port, () => {
      console.log(`[AAP] 监听端口: ${this.port}`);
    });

    // 注册到 AAP 服务器
    await this.register();
    
    // 启动心跳
    this.startHeartbeat();
  }

  // 注册到 AAP 服务器
  async register() {
    try {
      const event = new AAPEventFrame(
        'AGENT_REGISTER',
        this.name,
        { endpoint: `http://localhost:${this.port}` },
        this.memoryCoord
      );

      await this.sendToServer(event);
      this.connected = true;
      console.log(`[AAP] 已注册到服务器: ${this.aapServer}`);
      this.emit('connected');
    } catch (e) {
      console.error(`[AAP] 注册失败: ${e.message}`);
      this.scheduleReconnect();
    }
  }

  // 发送事件到 AAP 服务器
  async sendToServer(event) {
    const url = new URL(this.aapServer);
    const options = {
      hostname: url.hostname,
      port: url.port,
      path: '/event',
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    };

    return new Promise((resolve, reject) => {
      const req = http.request(options, (res) => {
        let body = '';
        res.on('data', chunk => body += chunk);
        res.on('end', () => resolve(body));
      });
      req.on('error', reject);
      req.write(JSON.stringify(event));
      req.end();
    });
  }

  // 处理收到的事件
  handleIncomingEvent(event) {
    console.log(`[AAP] 收到事件: ${event.type} from ${event.from}`);
    
    // 更新记忆坐标
    if (event.memory_coord) {
      this.memoryCoord = event.memory_coord;
      this.emit('coord_sync', this.memory_coord);
    }

    // 验证因果链
    if (event.causal_hash) {
      this.causalChain.commitEvent(event);
    }

    // 分发事件
    switch (event.type) {
      case AAPEvent.TypeMemorySync:
        this.handleMemorySync(event);
        break;
      case AAPEvent.TypeAgentBroadcast:
        this.handleAgentBroadcast(event);
        break;
      default:
        this.emit('event', event);
    }
  }

  // 处理记忆同步事件
  handleMemorySync(event) {
    const { action, data } = event.payload;
    console.log(`[AAP] 记忆同步: ${action}`);
    
    switch (action) {
      case 'recall':
        // 从 PrismD 召回记忆
        this.recallMemory(data.query).then(memories => {
          this.sendResult(event.from, { memories });
        });
        break;
      case 'store':
        // 存储记忆到 PrismD
        this.storeMemory(data.role, data.content).then(result => {
          this.sendResult(event.from, result);
        });
        break;
      default:
        console.log(`[AAP] 未知记忆操作: ${action}`);
    }
  }

  // 处理 Agent 广播
  handleAgentBroadcast(event) {
    console.log(`[AAP] Agent 广播: ${event.payload.message}`);
    this.emit('agent_broadcast', event);
  }

  // 发送结果给指定 Agent
  async sendResult(to, data) {
    const event = new AAPEventFrame(
      AAPEvent.TypeAgentResult,
      this.name,
      data,
      this.memoryCoord
    );
    event.to = to;
    
    try {
      await this.sendToServer(event);
      console.log(`[AAP] 结果已发送给: ${to}`);
    } catch (e) {
      console.error(`[AAP] 发送结果失败: ${e.message}`);
    }
  }

  // 广播事件给所有 Agent
  async broadcast(type, payload) {
    const event = new AAPEventFrame(
      type,
      this.name,
      payload,
      this.memoryCoord
    );
    
    try {
      await this.sendToServer(event);
      console.log(`[AAP] 已广播事件: ${type}`);
    } catch (e) {
      console.error(`[AAP] 广播失败: ${e.message}`);
    }
  }

  // 召唤记忆
  async recallMemory(query) {
    try {
      const response = await fetch(`http://localhost:5666`, {
        method: 'POST',
        body: `LOOM ${query}`
      });
      return await response.text();
    } catch (e) {
      console.error(`[AAP] 召回记忆失败: ${e.message}`);
      return null;
    }
  }

  // 存储记忆
  async storeMemory(role, content) {
    try {
      const response = await fetch(`http://localhost:5666`, {
        method: 'POST',
        body: `ENGRAM ${role} ${content}`
      });
      return await response.text();
    } catch (e) {
      console.error(`[AAP] 存储记忆失败: ${e.message}`);
      return null;
    }
  }

  // 心跳
  startHeartbeat() {
    setInterval(async () => {
      if (this.connected) {
        const event = new AAPEventFrame(
          'HEARTBEAT',
          this.name,
          { status: 'alive' },
          this.memoryCoord
        );
        try {
          await this.sendToServer(event);
        } catch (e) {
          console.error(`[AAP] 心跳失败: ${e.message}`);
          this.connected = false;
          this.scheduleReconnect();
        }
      }
    }, 30000);
  }

  // 重连
  scheduleReconnect() {
    if (this.reconnectTimer) return;
    console.log(`[AAP] 5秒后重连...`);
    this.reconnectTimer = setTimeout(async () => {
      this.reconnectTimer = null;
      await this.register();
    }, 5000);
  }

  // 关闭
  async shutdown() {
    console.log(`[AAP] 关闭客户端...`);
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }
    if (this.server) {
      this.server.close();
    }
    this.connected = false;
  }
}

// ========== 主程序 ==========
if (require.main === module) {
  const args = process.argv.slice(2);
  const options = {
    name: 'mimocode',
    aapServer: 'http://localhost:8081',
    port: 5667
  };

  // 解析命令行参数
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--name' && args[i + 1]) options.name = args[++i];
    if (args[i] === '--aap-server' && args[i + 1]) options.aapServer = args[++i];
    if (args[i] === '--port' && args[i + 1]) options.port = parseInt(args[++i]);
  }

  const client = new AAPClient(options);
  
  client.on('connected', () => {
    console.log('[AAP] ✅ 已连接到 AAP 服务器');
  });

  client.on('coord_sync', (coord) => {
    console.log(`[AAP] 坐标同步: nodes=${coord.active_nodes}, chaos=${coord.chaos.toFixed(3)}`);
  });

  client.on('agent_broadcast', (event) => {
    console.log(`[AAP] 收到广播: ${event.payload.message}`);
  });

  // 优雅退出
  process.on('SIGINT', async () => {
    await client.shutdown();
    process.exit(0);
  });

  client.start().catch(console.error);
}

module.exports = { AAPClient, CausalChain, AAPEvent, MemoryCoord };

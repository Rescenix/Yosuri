#!/usr/bin/env node
// aap-server.js — 轻量级 AAP 服务器
// 用法: node aap-server.js [--port 8081]

const http = require('http');
const crypto = require('crypto');

// ========== 事件定义 ==========
const AAPEvent = {
  TypeMemorySync: 'MEMORY_SYNC',
  TypeAgentBroadcast: 'AGENT_BROADCAST',
  TypeAgentResult: 'AGENT_RESULT',
  TypeChainHash: 'CHAIN_HASH'
};

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
    return this.hashChain.length > 0;
  }
}

// ========== AAP 服务器 ==========
class AAPServer {
  constructor(port = 8081) {
    this.port = port;
    this.agents = new Map();
    this.causalChain = new CausalChain();
    this.eventBus = [];
    this.server = null;
  }

  start() {
    this.server = http.createServer((req, res) => {
      if (req.method === 'POST' && req.url === '/event') {
        let body = '';
        req.on('data', chunk => body += chunk);
        req.on('end', () => {
          try {
            const event = JSON.parse(body);
            this.handleEvent(event);
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ ok: true }));
          } catch (e) {
            res.writeHead(400);
            res.end('Invalid event');
          }
        });
      } else if (req.method === 'GET' && req.url === '/status') {
        const agents = Array.from(this.agents.keys());
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          status: 'ok',
          agents: agents,
          chainLength: this.causalChain.hashChain.length
        }));
      } else {
        res.writeHead(404);
        res.end('Not found');
      }
    });

    this.server.listen(this.port, () => {
      console.log(`[AAP Server] 启动成功，监听端口: ${this.port}`);
      console.log(`[AAP Server] 状态查询: http://localhost:${this.port}/status`);
    });
  }

  handleEvent(event) {
    console.log(`[AAP Server] 收到事件: ${event.type} from ${event.from}`);

    // 验证因果链
    if (event.causal_hash) {
      this.causalChain.commitEvent(event);
    }

    // 处理注册
    if (event.type === 'AGENT_REGISTER') {
      this.registerAgent(event.from, event.payload.endpoint);
      return;
    }

    // 处理心跳
    if (event.type === 'HEARTBEAT') {
      console.log(`[AAP Server] 心跳: ${event.from}`);
      return;
    }

    // 广播给其他 Agent
    this.broadcast(event);
  }

  registerAgent(name, endpoint) {
    this.agents.set(name, {
      name: name,
      endpoint: endpoint,
      lastSeen: Date.now()
    });
    console.log(`[AAP Server] Agent 已注册: ${name} @ ${endpoint}`);
  }

  broadcast(event) {
    for (const [name, agent] of this.agents) {
      if (name === event.from) continue; // 不发给自己
      
      const url = new URL(agent.endpoint);
      const options = {
        hostname: url.hostname,
        port: url.port,
        path: '/event',
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      };

      const req = http.request(options, (res) => {
        let body = '';
        res.on('data', chunk => body += chunk);
        res.on('end', () => {
          console.log(`[AAP Server] 已发送给 ${name}: ${res.statusCode}`);
        });
      });

      req.on('error', (e) => {
        console.error(`[AAP Server] 发送失败 ${name}: ${e.message}`);
        // 标记为死亡
        this.agents.delete(name);
      });

      req.write(JSON.stringify(event));
      req.end();
    }
  }

  shutdown() {
    if (this.server) {
      this.server.close();
    }
  }
}

// ========== 主程序 ==========
if (require.main === module) {
  let port = 8081;
  const args = process.argv.slice(2);
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--port' && args[i + 1]) port = parseInt(args[++i]);
  }

  const server = new AAPServer(port);
  server.start();

  process.on('SIGINT', () => {
    console.log('\n[AAP Server] 正在关闭...');
    server.shutdown();
    process.exit(0);
  });
}

module.exports = { AAPServer, CausalChain, AAPEvent, AAPEventFrame };

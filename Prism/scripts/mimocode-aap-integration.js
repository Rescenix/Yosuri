#!/usr/bin/env node
// mimocode-aap-integration.js — MiMo Code AAP 集成示例
// 在 MiMo Code 中使用 AAP 协议存取记忆

const { AAPClient } = require('./aap-client.js');

// ========== 配置 ==========
const AAP_CONFIG = {
  name: 'mimocode',
  aapServer: 'http://localhost:8081',
  port: 5667
};

// ========== 初始化 AAP 客户端 ==========
let aapClient = null;

async function initAAP() {
  if (aapClient && aapClient.connected) {
    return aapClient;
  }

  aapClient = new AAPClient(AAP_CONFIG);
  
  aapClient.on('connected', () => {
    console.log('[AAP] ✅ MiMo Code 已连接到 AAP 总线');
  });

  aapClient.on('coord_sync', (coord) => {
    console.log(`[AAP] 坐标同步: nodes=${coord.active_nodes}`);
  });

  aapClient.on('agent_broadcast', (event) => {
    console.log(`[AAP] 收到广播: ${event.payload.message || JSON.stringify(event.payload)}`);
  });

  await aapClient.start();
  return aapClient;
}

// ========== 记忆操作 ==========

/**
 * 存储记忆到 PrismD
 * @param {string} role - 角色类型 (user/memory/reference)
 * @param {string} content - 记忆内容
 */
async function storeMemory(role, content) {
  const client = await initAAP();
  
  // 通过 AAP 广播记忆存储事件
  await client.broadcast('MEMORY_SYNC', {
    action: 'store',
    data: { role, content }
  });
  
  console.log(`[AAP] 记忆已存储: role=${role}, content=${content.substring(0, 50)}...`);
}

/**
 * 从 PrismD 召回记忆
 * @param {string} query - 查询关键词
 * @returns {Promise<string>} 召回的记忆
 */
async function recallMemory(query) {
  const client = await initAAP();
  
  // 直接调用 PrismD
  try {
    const response = await fetch('http://localhost:5666', {
      method: 'POST',
      body: `LOOM ${query}`
    });
    return await response.text();
  } catch (e) {
    console.error(`[AAP] 召回失败: ${e.message}`);
    return null;
  }
}

/**
 * 同步 Claude 记忆文件到 PrismD
 * @param {string} filePath - memory/*.md 文件路径
 */
async function syncClaudeMemory(filePath) {
  const fs = require('fs');
  const path = require('path');
  
  if (!fs.existsSync(filePath)) {
    console.error(`[AAP] 文件不存在: ${filePath}`);
    return;
  }
  
  const content = fs.readFileSync(filePath, 'utf-8');
  
  // 解析 frontmatter
  let name = path.basename(filePath, '.md');
  let description = '';
  let type = 'reference';
  
  const fmMatch = content.match(/^---\s*\n([\s\S]*?)\n---/);
  if (fmMatch) {
    const fm = fmMatch[1];
    const nameMatch = fm.match(/^name:\s*(.+)$/m);
    const descMatch = fm.match(/^description:\s*(.+)$/m);
    const typeMatch = fm.match(/^type:\s*(.+)$/m);
    
    if (nameMatch) name = nameMatch[1].trim();
    if (descMatch) description = descMatch[1].trim();
    if (typeMatch) type = typeMatch[1].trim();
  }
  
  // 提取正文（去掉 frontmatter）
  const body = content.replace(/^---[\s\S]*?\n---\s*\n?/, '').trim();
  
  // 存储到 PrismD
  const text = `${description}: ${body.substring(0, 300)}`;
  await storeMemory(`${type}-memory`, text);
  
  console.log(`[AAP] 已同步: ${name} -> PrismD`);
}

// ========== 主程序 ==========
if (require.main === module) {
  const args = process.argv.slice(2);
  const command = args[0];
  
  (async () => {
    switch (command) {
      case 'store':
        if (args.length < 3) {
          console.log('用法: node mimocode-aap-integration.js store <role> <content>');
          process.exit(1);
        }
        await storeMemory(args[1], args[2]);
        break;
        
      case 'recall':
        if (args.length < 2) {
          console.log('用法: node mimocode-aap-integration.js recall <query>');
          process.exit(1);
        }
        const result = await recallMemory(args[1]);
        console.log(result);
        break;
        
      case 'sync':
        if (args.length < 2) {
          console.log('用法: node mimocode-aap-integration.js sync <file-path>');
          process.exit(1);
        }
        await syncClaudeMemory(args[1]);
        break;
        
      default:
        console.log('用法:');
        console.log('  node mimocode-aap-integration.js store <role> <content>');
        console.log('  node mimocode-aap-integration.js recall <query>');
        console.log('  node mimocode-aap-integration.js sync <file-path>');
        break;
    }
    
    // 关闭连接
    if (aapClient) {
      await aapClient.shutdown();
    }
  })();
}

module.exports = { storeMemory, recallMemory, syncClaudeMemory };

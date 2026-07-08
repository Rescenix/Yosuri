// 解析工具调用参数字符串（形如 path="a.py" content="..."）成对象。
// ToolActionRow/ChatWidget 都需要从同一份 args 字符串里读取字段，抽成共享 util 避免各自维护一份正则。
export function parseToolArgs(argsStr) {
  const args = {}
  const re = /(\w+)="([\s\S]*?)"/g
  let m
  while ((m = re.exec(argsStr || '')) !== null) args[m[1]] = m[2]
  return args
}

export function fileBaseName(path) {
  if (!path) return '文件'
  return path.split(/[\\/]/).pop()
}

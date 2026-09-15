let backendBase = ''

function shouldRouteToBackend(raw) {
  return raw === '/api' || raw.startsWith('/api/') ||
    raw === '/images' || raw.startsWith('/images/')
}

export function backendURL(raw) {
  if (!backendBase || typeof raw !== 'string' || !shouldRouteToBackend(raw)) return raw
  return backendBase + raw
}

async function resolveDesktopBackend() {
  const binding = globalThis.go?.main?.DesktopApp?.BackendURL
  if (typeof binding !== 'function') return ''
  try {
    return String(await binding()).replace(/\/+$/, '')
  } catch (error) {
    console.error('读取桌面 API 地址失败', error)
    return ''
  }
}

function installFetchBridge() {
  const nativeFetch = globalThis.fetch.bind(globalThis)
  globalThis.fetch = (input, init) => {
    if (typeof input === 'string') return nativeFetch(backendURL(input), init)
    if (input instanceof URL) return nativeFetch(new URL(backendURL(input.toString())), init)
    if (input instanceof Request && shouldRouteToBackend(new URL(input.url).pathname)) {
      const url = new URL(input.url)
      return nativeFetch(new Request(backendBase + url.pathname + url.search, input), init)
    }
    return nativeFetch(input, init)
  }
}

function installEventSourceBridge() {
	const NativeEventSource = globalThis.EventSource
	if (!NativeEventSource) return
	function DesktopEventSource(url, options) {
		return new NativeEventSource(backendURL(String(url)), options)
	}
	DesktopEventSource.prototype = NativeEventSource.prototype
	DesktopEventSource.CONNECTING = NativeEventSource.CONNECTING
	DesktopEventSource.OPEN = NativeEventSource.OPEN
	DesktopEventSource.CLOSED = NativeEventSource.CLOSED
	globalThis.EventSource = DesktopEventSource
}

function installWebSocketBridge() {
	const NativeWebSocket = globalThis.WebSocket
	if (!NativeWebSocket) return
	function DesktopWebSocket(url, protocols) {
		// 相对路径 → 后端绝对路径；http→ws 转换（fetch bridge 返回 http://，WebSocket 需要 ws://）
		let target = backendURL(String(url))
		if (target.startsWith('http://')) {
			target = 'ws://' + target.slice(7)
		} else if (target.startsWith('https://')) {
			target = 'wss://' + target.slice(8)
		}
		return new NativeWebSocket(target, protocols)
	}
	DesktopWebSocket.prototype = NativeWebSocket.prototype
	DesktopWebSocket.CONNECTING = NativeWebSocket.CONNECTING
	DesktopWebSocket.OPEN = NativeWebSocket.OPEN
	DesktopWebSocket.CLOSING = NativeWebSocket.CLOSING
	DesktopWebSocket.CLOSED = NativeWebSocket.CLOSED
	globalThis.WebSocket = DesktopWebSocket
}

// 显式解析后端绝对地址（memoized）。给「不能依赖全局 fetch 桥」的调用方用：
// 桥的改写要等 installDesktopTransport 完成才生效，且个别 WebView2 环境下绑定/时序
// 不稳，相对路径会直接打到 https://wails.localhost 的 AssetServer 上 404（前缀洗不掉）。
// Web 部署没有 Wails 绑定 → 返回空串，相对路径行为不变。
let resolvedBase = null
export async function getBackendBase() {
  if (resolvedBase !== null) return resolvedBase
  resolvedBase = await resolveDesktopBackend()
  if (!resolvedBase) resolvedBase = globalThis.__RESCENE_BACKEND_URL__ || ''
  return resolvedBase
}

export async function installDesktopTransport() {
	backendBase = await resolveDesktopBackend()
	globalThis.__RESCENE_BACKEND_URL__ = backendBase
	if (!backendBase) return
	installFetchBridge()
	installEventSourceBridge()
	installWebSocketBridge()
}

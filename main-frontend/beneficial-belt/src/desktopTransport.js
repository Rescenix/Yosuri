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

export async function installDesktopTransport() {
  backendBase = await resolveDesktopBackend()
  globalThis.__RESCENE_BACKEND_URL__ = backendBase
  if (!backendBase) return
  installFetchBridge()
  installEventSourceBridge()
}

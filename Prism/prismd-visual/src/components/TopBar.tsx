export default function TopBar() {
  return (
    <header style={{ flex: 'none', height: 52, display: 'flex', alignItems: 'center', gap: 16, padding: '0 16px 0 14px', background: '#ffffff', borderBottom: '1px solid #e4e7ee', zIndex: 30 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <svg width="24" height="24" viewBox="0 0 24 24" style={{ display: 'block' }}>
          <polygon points="12,3 21,19 3,19" fill="none" stroke="#12151c" strokeWidth={1.6} strokeLinejoin="round" />
          <circle cx="9.2" cy="15.4" r="1.5" fill="#4f7ef0" />
          <circle cx="12" cy="10.6" r="1.5" fill="#0fb5a0" />
          <circle cx="14.8" cy="15.4" r="1.5" fill="#e8920c" />
        </svg>
        <div style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.05 }}>
          <span style={{ fontWeight: 700, fontSize: 14, letterSpacing: 0.5 }}>PRISM</span>
          <span style={{ fontSize: 9.5, letterSpacing: 1.4, color: '#8a93a6', fontFamily: "'IBM Plex Mono',monospace", textTransform: 'uppercase' }}>Memory Field</span>
        </div>
      </div>
      <div style={{ width: 1, height: 22, background: '#e4e7ee' }} />
      <div style={{ fontSize: 12, color: '#565f70', fontFamily: "'IBM Plex Mono',monospace" }}>记忆场拓扑 · 突触图谱</div>
      <div style={{ flex: 1 }} />
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, fontFamily: "'IBM Plex Mono',monospace", fontSize: 11, color: '#565f70', padding: '5px 10px', border: '1px solid #e4e7ee', borderRadius: 7 }}>
        <span style={{ width: 7, height: 7, borderRadius: '50%', background: '#12a594', animation: 'prism-live 2s ease-in-out infinite' }} />
        <span>PrismD</span>
        <span style={{ color: '#aab0bd' }}>:5666</span>
      </div>
    </header>
  )
}

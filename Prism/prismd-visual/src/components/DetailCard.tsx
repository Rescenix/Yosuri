import { CMAP, DEFAULT_CLUSTER_COLOR, EMO, EVT, energyFill, mixHex, short } from '../lib/colors'
import { effectiveValue, formatLastAccess } from '../lib/decay'
import type { GraphController } from '../hooks/useGraphController'

interface Props {
  ctrl: GraphController
}

export default function DetailCard({ ctrl }: Props) {
  const { selectedId, nodeById, adj, closeCard, onCopy, copied, traceDiffusion, selectNode } = ctrl
  if (selectedId == null) return null
  const n = nodeById.get(selectedId)
  if (!n) return null

  const now = Date.now()
  const c = CMAP[n.cluster]?.color || DEFAULT_CLUSTER_COLOR
  const e = effectiveValue(n.energy, n.decayRate, n.lastAccessAt, now)
  const state = e >= 0.7 ? { t: '活跃', c: '#12a594' } : e < 0.24 ? { t: '休眠 · 衰减', c: '#aab0bd' } : { t: '空闲', c: '#e8920c' }
  const emo = EMO[n.emotion] || EMO.neutral

  const pts: string[] = []
  for (let i = 0; i < 12; i++) {
    const proj = e * Math.exp(-n.decayRate * i * 3.2)
    const x = (i / 11) * 94 + 1
    const y = 28 - proj * 24
    pts.push(`${x.toFixed(1)},${y.toFixed(1)}`)
  }

  const neighbors = [...(adj.get(selectedId) || [])]
    .sort((a, b) => b.weight - a.weight)
    .slice(0, 8)
    .map((nb) => {
      const nn = nodeById.get(nb.id)
      if (!nn) return null
      const nc = CMAP[nn.cluster]?.color || DEFAULT_CLUSTER_COLOR
      const ne = effectiveValue(nn.energy, nn.decayRate, nn.lastAccessAt, now)
      return { id: nb.id, content: short(nn.content, 26), weight: nb.weight.toFixed(2), color: nc, dotBg: energyFill(nc, ne) }
    })
    .filter((x): x is NonNullable<typeof x> => x != null)

  return (
    <aside
      key={n.id}
      style={{
        position: 'absolute', top: 14, right: 14, bottom: 14, width: 346, background: 'rgba(255,255,255,.94)', backdropFilter: 'blur(10px)',
        border: '1px solid #e4e7ee', borderRadius: 14, boxShadow: '0 8px 34px rgba(20,30,60,.14)', display: 'flex', flexDirection: 'column', overflow: 'hidden', zIndex: 15,
        animation: 'prism-card-in .32s cubic-bezier(.16,1,.3,1) both',
      }}
    >
      <div style={{ flex: 'none', padding: '15px 16px 13px', borderBottom: '1px solid #eef0f4' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 11 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
            <span style={{ width: 11, height: 11, borderRadius: '50%', background: energyFill(c, e), border: `2px solid ${c}` }} />
            <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 12, color: '#565f70' }}>{n.cluster}</span>
            <span style={{ fontSize: 11, color: '#fff', background: n.role === 'user' ? '#4f7ef0' : '#12151c', padding: '2px 8px', borderRadius: 11 }}>{n.role === 'user' ? '用户' : '助手'}</span>
          </div>
          <button onClick={closeCard} style={{ border: 'none', background: '#f2f3f7', width: 26, height: 26, borderRadius: 8, cursor: 'pointer', color: '#565f70', fontSize: 14 }}>✕</button>
        </div>
        <div style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 22, fontWeight: 600, color: '#12151c', lineHeight: 1 }}>#{n.id}</div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '15px 16px 18px' }}>
        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginBottom: 7 }}>
          <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 10, letterSpacing: 1, color: '#9aa1b0', textTransform: 'uppercase' }}>能量 Energy</span>
          <span style={{ fontSize: 12, fontWeight: 600, color: state.c }}>{state.t}</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 11 }}>
          <div style={{ flex: 1, height: 9, borderRadius: 6, background: '#eef0f4', overflow: 'hidden' }}>
            <div style={{ height: '100%', width: `${Math.round(e * 100)}%`, background: `linear-gradient(90deg,${mixHex('#eef0f4', c, 0.4)},${c})`, borderRadius: 6 }} />
          </div>
          <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 15, fontWeight: 600, width: 44, textAlign: 'right' }}>{e.toFixed(2)}</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px', background: '#f7f8fb', border: '1px solid #eef0f4', borderRadius: 10, marginBottom: 16 }}>
          <svg width="96" height="30" viewBox="0 0 96 30" style={{ flex: 'none' }}>
            <polyline points={pts.join(' ')} fill="none" stroke="#4f7ef0" strokeWidth={1.6} strokeLinejoin="round" strokeLinecap="round" />
          </svg>
          <div style={{ flex: 1, fontFamily: "'IBM Plex Mono',monospace", fontSize: 10.5, color: '#8a93a6', lineHeight: 1.7 }}>
            <div>衰减 <span style={{ color: '#3a4150' }}>{n.decayRate.toFixed(3)}</span>/h</div>
            <div>访问 <span style={{ color: '#3a4150' }}>{formatLastAccess(n.lastAccessAt, now)}</span></div>
          </div>
        </div>

        <div style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 10, letterSpacing: 1, color: '#9aa1b0', textTransform: 'uppercase', marginBottom: 8 }}>情感状态 Affect</div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 7, marginBottom: 17 }}>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 11px', borderRadius: 9, fontSize: 11.5, fontWeight: 500, color: '#fff', background: emo.c }}>{emo.t}</span>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 10px', borderRadius: 9, background: '#f2f3f7', fontSize: 11.5, color: '#3a4150' }}>
            强度<span style={{ fontFamily: "'IBM Plex Mono',monospace", fontWeight: 600 }}>{n.intensity.toFixed(2)}</span>
          </span>
          <span style={{ display: 'inline-flex', alignItems: 'center', padding: '4px 10px', borderRadius: 9, background: '#f2f3f7', fontSize: 11.5, color: '#3a4150' }}>{EVT[n.eventType] || n.eventType}</span>
        </div>

        <div style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 10, letterSpacing: 1, color: '#9aa1b0', textTransform: 'uppercase', marginBottom: 8 }}>内容 Content</div>
        <div style={{ fontSize: 13.5, lineHeight: 1.7, color: '#22283a', padding: '11px 13px', background: '#f7f8fb', border: '1px solid #eef0f4', borderRadius: 10, marginBottom: 17 }}>{n.content}</div>

        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginBottom: 9 }}>
          <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 10, letterSpacing: 1, color: '#9aa1b0', textTransform: 'uppercase' }}>突触连接 Synapses</span>
          <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 11, color: '#aab0bd' }}>{adj.get(selectedId)?.length || 0}</span>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          {neighbors.map((nb) => (
            <div key={nb.id} onClick={() => selectNode(nb.id)} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', border: '1px solid #eef0f4', borderRadius: 9, cursor: 'pointer', background: '#fff' }}>
              <span style={{ flex: 'none', width: 9, height: 9, borderRadius: '50%', background: nb.dotBg, border: `1.5px solid ${nb.color}` }} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 12, color: '#22283a', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{nb.content}</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 4 }}>
                  <div style={{ flex: 1, height: 3, borderRadius: 3, background: '#eef0f4', overflow: 'hidden' }}>
                    <div style={{ height: '100%', width: `${Math.round(+nb.weight * 100)}%`, background: nb.color, borderRadius: 3 }} />
                  </div>
                  <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 9.5, color: '#9aa1b0' }}>{nb.weight}</span>
                </div>
              </div>
              <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 10, color: '#c3c9d4' }}>#{nb.id}</span>
            </div>
          ))}
        </div>
      </div>

      <div style={{ flex: 'none', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1, background: '#eef0f4', borderTop: '1px solid #eef0f4' }}>
        <button onClick={() => traceDiffusion(n.id)} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3, padding: '10px 2px', border: 'none', background: '#fff', cursor: 'pointer', color: '#6d5efc', fontFamily: 'inherit' }}>
          <span style={{ fontSize: 14 }}>⇢</span><span style={{ fontSize: 9.5 }}>追踪扩散</span>
        </button>
        <button onClick={onCopy} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3, padding: '10px 2px', border: 'none', background: '#fff', cursor: 'pointer', color: '#3a4150', fontFamily: 'inherit' }}>
          <span style={{ fontSize: 14 }}>{copied ? '✓' : '⧉'}</span><span style={{ fontSize: 9.5 }}>{copied ? '已复制' : '复制'}</span>
        </button>
      </div>
    </aside>
  )
}

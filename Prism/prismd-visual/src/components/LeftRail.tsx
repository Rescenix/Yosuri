import { CLUSTERS, energyFill } from '../lib/colors'
import type { NeuronData } from '../types'
import type { PhysicsState } from '../hooks/useGraphController'

interface Props {
  stats: { nodes: number; edges: number; active: number; dormant: number }
  neurons: NeuronData[]
  activeCluster: string | null
  toggleCluster: (key: string) => void
  onClearFilter: () => void
  physics: PhysicsState
  onRepulsion: (v: number) => void
  onEdgeLength: (v: number) => void
  onGravity: (v: number) => void
  onResetLayout: () => void
}

const statCard = { border: '1px solid #e9ebf1', borderRadius: 9, padding: '9px 11px' } as const
const statValue = { fontSize: 20, fontWeight: 600, fontFamily: "'IBM Plex Mono',monospace", lineHeight: 1 } as const
const statLabel = { fontSize: 10.5, color: '#8a93a6', marginTop: 3 } as const
const sectionLabel = { fontFamily: "'IBM Plex Mono',monospace", fontSize: 10, letterSpacing: 1.2, color: '#9aa1b0', textTransform: 'uppercase' as const }

export default function LeftRail({ stats, neurons, activeCluster, toggleCluster, onClearFilter, physics, onRepulsion, onEdgeLength, onGravity, onResetLayout }: Props) {
  return (
    <aside style={{ flex: 'none', width: 274, background: '#ffffff', borderRight: '1px solid #e4e7ee', display: 'flex', flexDirection: 'column', minHeight: 0, zIndex: 20 }}>
      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 15px 20px' }}>
        <div style={{ ...sectionLabel, marginBottom: 10 }}>场态 · Field</div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 22 }}>
          <div style={statCard}>
            <div style={statValue}>{stats.nodes}</div>
            <div style={statLabel}>神经元 nodes</div>
          </div>
          <div style={statCard}>
            <div style={statValue}>{stats.edges}</div>
            <div style={statLabel}>突触 synapses</div>
          </div>
          <div style={statCard}>
            <div style={{ ...statValue, color: '#12a594' }}>{stats.active}</div>
            <div style={statLabel}>活跃 active</div>
          </div>
          <div style={statCard}>
            <div style={{ ...statValue, color: '#aab0bd' }}>{stats.dormant}</div>
            <div style={statLabel}>休眠 dormant</div>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginBottom: 9 }}>
          <span style={sectionLabel}>簇 · Cluster 描边</span>
          <span onClick={onClearFilter} style={{ fontSize: 10.5, color: '#5b6472', cursor: 'pointer', userSelect: 'none' }}>全部</span>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 5, marginBottom: 22 }}>
          {CLUSTERS.map((c) => {
            const count = neurons.filter((n) => n.cluster === c.key).length
            const on = activeCluster === c.key
            return (
              <div
                key={c.key}
                onClick={() => toggleCluster(c.key)}
                style={{
                  display: 'flex', alignItems: 'center', gap: 10, padding: '7px 10px', borderRadius: 9, cursor: 'pointer', userSelect: 'none',
                  border: `1px solid ${on ? c.color + '8c' : '#eceef2'}`,
                  background: on ? c.color + '17' : '#fff',
                }}
              >
                <span style={{ flex: 'none', width: 12, height: 12, borderRadius: '50%', background: energyFill(c.color, 0.5), border: `2px solid ${c.color}` }} />
                <span style={{ flex: 1, fontSize: 12.5, fontWeight: 500 }}>{c.name}</span>
                <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 11, color: '#9aa1b0' }}>{count}</span>
              </div>
            )
          })}
        </div>

        <div style={{ height: 1, background: '#eceef2', margin: '8px 0 18px' }} />

        <div style={{ ...sectionLabel, marginBottom: 13 }}>力导向物理 · Force</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 15, marginBottom: 16 }}>
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11.5, marginBottom: 6 }}>
              <span style={{ color: '#5b6472' }}>斥力 repulsion</span>
              <span style={{ fontFamily: "'IBM Plex Mono',monospace", color: '#12151c' }}>{physics.repulsion}</span>
            </div>
            <input type="range" min={60} max={600} step={10} value={physics.repulsion} onChange={(e) => onRepulsion(+e.target.value)} style={{ width: '100%' }} />
          </div>
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11.5, marginBottom: 6 }}>
              <span style={{ color: '#5b6472' }}>边长 edgeLength</span>
              <span style={{ fontFamily: "'IBM Plex Mono',monospace", color: '#12151c' }}>{physics.edgeLength}</span>
            </div>
            <input type="range" min={30} max={300} step={5} value={physics.edgeLength} onChange={(e) => onEdgeLength(+e.target.value)} style={{ width: '100%' }} />
          </div>
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11.5, marginBottom: 6 }}>
              <span style={{ color: '#5b6472' }}>引力 gravity</span>
              <span style={{ fontFamily: "'IBM Plex Mono',monospace", color: '#12151c' }}>{physics.gravity.toFixed(2)}</span>
            </div>
            <input type="range" min={0} max={0.5} step={0.01} value={physics.gravity} onChange={(e) => onGravity(+e.target.value)} style={{ width: '100%' }} />
          </div>
        </div>
        <button onClick={onResetLayout} style={{ width: '100%', padding: 8, border: '1px solid #e0e3ea', borderRadius: 8, background: '#f7f8fb', color: '#12151c', fontSize: 12, fontWeight: 500, cursor: 'pointer', fontFamily: 'inherit' }}>↺ 重置布局</button>
      </div>
    </aside>
  )
}

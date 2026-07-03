import type { GraphController } from '../hooks/useGraphController'
import DetailCard from './DetailCard'

interface Props {
  ctrl: GraphController
}

export default function GraphCanvas({ ctrl }: Props) {
  const { chartRef, selectedId, tracing, menu, traceStart, traceHop, traceMaxHop, tracePlaying, nodeById, visibleCount, stats } = ctrl
  const showHint = selectedId == null && !tracing
  const traceStartNode = traceStart != null ? nodeById.get(traceStart) : null
  const traceLabel = traceStartNode ? `扩散追踪 · 源 #${traceStartNode.id} · 第 ${traceHop}/${traceMaxHop} 跳` : '扩散追踪'

  return (
    <main style={{ position: 'relative', flex: 1, minWidth: 0, background: '#f4f6f9', backgroundImage: 'radial-gradient(circle,rgba(30,41,80,.055) 1px,transparent 1px)', backgroundSize: '22px 22px' }}>
      <div ref={chartRef} style={{ position: 'absolute', inset: 0 }} />

      <div style={{ position: 'absolute', right: 16, bottom: 16, pointerEvents: 'none', fontFamily: "'IBM Plex Mono',monospace", fontSize: 10.5, color: '#9aa1b0', background: 'rgba(255,255,255,.72)', padding: '4px 10px', borderRadius: 14, border: '1px solid #e4e7ee' }}>
        已展开 {visibleCount} / {stats.nodes}
      </div>

      {showHint && (
        <div style={{ position: 'absolute', left: '50%', bottom: 22, transform: 'translateX(-50%)', pointerEvents: 'none', fontFamily: "'IBM Plex Mono',monospace", fontSize: 11, color: '#9aa1b0', background: 'rgba(255,255,255,.72)', padding: '5px 12px', borderRadius: 20, border: '1px solid #e4e7ee' }}>
          点击节点展开邻居 · 右键操作 · 滚轮缩放 · 拖拽平移
        </div>
      )}

      <div style={{ position: 'absolute', left: 16, bottom: 16, display: 'flex', flexDirection: 'column', gap: 1, background: '#fff', border: '1px solid #e4e7ee', borderRadius: 9, overflow: 'hidden', boxShadow: '0 2px 8px rgba(20,30,60,.07)' }}>
        <button onClick={ctrl.onZoomIn} style={{ width: 34, height: 32, border: 'none', borderBottom: '1px solid #eceef2', background: '#fff', cursor: 'pointer', fontSize: 17, color: '#3a4150' }}>+</button>
        <button onClick={ctrl.onZoomOut} style={{ width: 34, height: 32, border: 'none', borderBottom: '1px solid #eceef2', background: '#fff', cursor: 'pointer', fontSize: 17, color: '#3a4150' }}>−</button>
        <button onClick={ctrl.onFit} style={{ width: 34, height: 32, border: 'none', background: '#fff', cursor: 'pointer', fontSize: 13, color: '#3a4150' }} title="适应视图">⤢</button>
      </div>

      {tracing && (
        <div style={{ position: 'absolute', left: '50%', top: 16, transform: 'translateX(-50%)', display: 'flex', alignItems: 'center', gap: 8, background: '#fff', border: '1px solid #ded9ff', boxShadow: '0 4px 16px rgba(109,94,252,.18)', borderRadius: 22, padding: '7px 8px 7px 14px' }}>
          <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#6d5efc', animation: 'prism-live 1.2s ease-in-out infinite' }} />
          <span style={{ fontSize: 12, color: '#3a4150', whiteSpace: 'nowrap' }}>{traceLabel}</span>
          <div style={{ width: 1, height: 16, background: '#eef0f4' }} />
          <button
            onClick={ctrl.onTracePrev}
            disabled={traceHop <= 0}
            style={{ border: 'none', width: 24, height: 24, borderRadius: '50%', background: '#f3f1ff', color: '#6d5efc', fontSize: 12, fontFamily: 'inherit', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: traceHop <= 0 ? 'default' : 'pointer', opacity: traceHop <= 0 ? 0.35 : 1 }}
          >‹</button>
          <button onClick={ctrl.onTracePlayToggle} style={{ border: 'none', width: 26, height: 26, borderRadius: '50%', background: '#6d5efc', color: '#fff', fontSize: 11, fontFamily: 'inherit', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' }}>
            {tracePlaying ? '⏸' : '▶'}
          </button>
          <button
            onClick={ctrl.onTraceNext}
            disabled={traceHop >= traceMaxHop}
            style={{ border: 'none', width: 24, height: 24, borderRadius: '50%', background: '#f3f1ff', color: '#6d5efc', fontSize: 12, fontFamily: 'inherit', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: traceHop >= traceMaxHop ? 'default' : 'pointer', opacity: traceHop >= traceMaxHop ? 0.35 : 1 }}
          >›</button>
          <button onClick={ctrl.stopTrace} style={{ border: 'none', background: '#f3f1ff', color: '#6d5efc', fontSize: 11, fontWeight: 600, padding: '4px 11px', borderRadius: 16, cursor: 'pointer', fontFamily: 'inherit' }}>结束追踪</button>
        </div>
      )}

      {menu.show && menu.id != null && (
        <div style={{ position: 'fixed', left: menu.x, top: menu.y, minWidth: 186, background: '#fff', border: '1px solid #e4e7ee', borderRadius: 11, boxShadow: '0 10px 30px rgba(20,30,60,.16)', padding: 5, zIndex: 60, overflow: 'hidden' }} onMouseDown={(e) => e.stopPropagation()}>
          <div style={{ padding: '8px 13px 7px', borderBottom: '1px solid #f0f1f5', fontFamily: "'IBM Plex Mono',monospace", fontSize: 10.5, color: '#9aa1b0' }}>节点 #{menu.id}</div>
          <button onClick={() => ctrl.traceDiffusion(menu.id!)} style={{ display: 'flex', gap: 9, alignItems: 'center', width: '100%', padding: '9px 13px', border: 'none', background: 'none', cursor: 'pointer', fontSize: 12.5, color: '#1c2230', textAlign: 'left', fontFamily: 'inherit' }}>
            <span style={{ color: '#6d5efc', width: 15 }}>⇢</span>追踪扩散激活
          </button>
          <button onClick={() => ctrl.selectNode(menu.id!)} style={{ display: 'flex', gap: 9, alignItems: 'center', width: '100%', padding: '9px 13px', border: 'none', background: 'none', cursor: 'pointer', fontSize: 12.5, color: '#1c2230', textAlign: 'left', fontFamily: 'inherit' }}>
            <span style={{ color: '#565f70', width: 15 }}>▤</span>查看完整记忆 / 复制
          </button>
        </div>
      )}

      <DetailCard ctrl={ctrl} />
    </main>
  )
}

import { useUIState } from '../store/useUIState'

const EMOTION_COLORS: Record<string, string> = {
  neutral: '#c0d0ff', happy: '#ffd700', angry: '#ff4444',
  sad: '#9966ff', anxious: '#ff9900',
}

export default function MemoryPanel({ onClose }: { onClose: () => void }) {
  // ★ 只取 closePanel 方法
  const { selectedNode, closePanel } = useUIState()

  const handleClose = () => {
    onClose()
    closePanel() // 干净关闭，绝不移动相机
  }

  if (!selectedNode) return null

  const emotionColor = EMOTION_COLORS[selectedNode.emotion || 'neutral'] || '#7aa2f7'

  return (
    <div style={{
      position: 'absolute', right: 20, top: '20%', width: 360,
      background: 'rgba(8, 10, 24, 0.85)', backdropFilter: 'blur(16px)',
      borderLeft: `4px solid ${emotionColor}`,
      boxShadow: `0 0 30px ${emotionColor}30, inset 0 0 20px rgba(255,255,255,0.02)`,
      padding: 20, borderRadius: 8, color: '#edf2ff', fontFamily: 'monospace',
      animation: 'slideInRight 0.5s cubic-bezier(0.34, 1.56, 0.64, 1) forwards',
      transform: 'translateX(120%)'
    }}>
      <style>{`
        @keyframes slideInRight {
          from { transform: translateX(120%); opacity: 0; }
          to { transform: translateX(0%); opacity: 1; }
        }
      `}</style>
      <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: `1px solid ${emotionColor}40`, paddingBottom: 10 }}>
        <span style={{ color: emotionColor }}>⚡ 记忆信号源</span>
        <button onClick={handleClose} style={{ background:'none', border:'none', color:'#7aa2f7', cursor:'pointer' }}>✕</button>
      </div>
      <div style={{ marginTop: 12 }}>
        <div style={{ fontSize: 12, color: '#90a4d4' }}>ID: {selectedNode.id} · {selectedNode.role} · 能量 {selectedNode.conductance.toFixed(2)}</div>
        <div style={{ fontSize: 14, marginTop: 8, lineHeight: 1.6, color: '#ffffff' }}>{selectedNode.content}</div>
      </div>
    </div>
  )
}
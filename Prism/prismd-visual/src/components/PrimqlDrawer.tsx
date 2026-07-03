import type { GraphController } from '../hooks/useGraphController'

interface Props {
  ctrl: GraphController
}

export default function PrimqlDrawer({ ctrl }: Props) {
  const { consoleOpen, toggleConsole, primqlResult, primqlInput, setPrimqlInput, runPrimql } = ctrl

  return (
    <div style={{ flex: 'none', display: 'flex', flexDirection: 'column', background: '#fff', borderTop: '1px solid #e4e7ee', zIndex: 25, height: consoleOpen ? 220 : 38, transition: 'height .22s cubic-bezier(.4,0,.2,1)', overflow: 'hidden' }}>
      <div onClick={toggleConsole} style={{ flex: 'none', height: 38, display: 'flex', alignItems: 'center', gap: 10, padding: '0 16px', cursor: 'pointer', borderBottom: '1px solid #eef0f4', background: '#fff' }}>
        <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 12, fontWeight: 600, color: '#12151c' }}>PrimQL</span>
        <span style={{ fontFamily: "'IBM Plex Mono',monospace", fontSize: 11, color: '#9aa1b0' }}>原语控制台 · POST :5666</span>
        <div style={{ flex: 1 }} />
        <span style={{ fontSize: 12, color: '#9aa1b0', transform: consoleOpen ? 'rotate(0deg)' : 'rotate(180deg)', display: 'inline-block' }}>▴</span>
      </div>
      {consoleOpen && (
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0, background: '#fbfcfd' }}>
          <div style={{ flex: 1, overflowY: 'auto', padding: '12px 16px', fontFamily: "'IBM Plex Mono',monospace", fontSize: 12, lineHeight: 1.65, color: '#3a4150', whiteSpace: 'pre-wrap' }}>{primqlResult}</div>
          <div style={{ flex: 'none', display: 'flex', gap: 8, padding: '10px 16px 12px', borderTop: '1px solid #eef0f4' }}>
            <span style={{ fontFamily: "'IBM Plex Mono',monospace", color: '#6d5efc', fontSize: 13, alignSelf: 'center' }}>›</span>
            <input
              value={primqlInput}
              onChange={(e) => setPrimqlInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  runPrimql()
                }
              }}
              placeholder="STATS FULL · GRAPH · TRACE 5 · HELP"
              style={{ flex: 1, padding: '7px 11px', border: '1px solid #e4e7ee', borderRadius: 8, background: '#fff', color: '#12151c', fontFamily: "'IBM Plex Mono',monospace", fontSize: 12.5, outline: 'none' }}
            />
            <button onClick={runPrimql} style={{ padding: '7px 18px', border: 'none', borderRadius: 8, background: '#12151c', color: '#fff', fontFamily: 'inherit', fontSize: 12, fontWeight: 500, cursor: 'pointer' }}>运行 ▶</button>
          </div>
        </div>
      )}
    </div>
  )
}

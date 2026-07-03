import { usePrismData } from './hooks/usePrismData'
import { useGraphController } from './hooks/useGraphController'
import TopBar from './components/TopBar'
import LeftRail from './components/LeftRail'
import GraphCanvas from './components/GraphCanvas'
import PrimqlDrawer from './components/PrimqlDrawer'

export default function App() {
  const { neurons, edges } = usePrismData()
  const ctrl = useGraphController(neurons, edges)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', width: '100vw', overflow: 'hidden', background: '#eceef3' }}>
      <TopBar />
      <div style={{ flex: 1, display: 'flex', minHeight: 0 }}>
        <LeftRail
          stats={ctrl.stats}
          neurons={neurons}
          activeCluster={ctrl.activeCluster}
          toggleCluster={ctrl.toggleCluster}
          onClearFilter={ctrl.onClearFilter}
          physics={ctrl.physics}
          onRepulsion={ctrl.onRepulsion}
          onEdgeLength={ctrl.onEdgeLength}
          onGravity={ctrl.onGravity}
          onResetLayout={ctrl.onResetLayout}
        />
        <GraphCanvas ctrl={ctrl} />
      </div>
      <PrimqlDrawer ctrl={ctrl} />
    </div>
  )
}

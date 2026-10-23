import { motion } from 'motion/react'
import { Link } from 'react-router'
import { PillIcon } from '../components/PillIcon'
import { Page } from '../components/ui'

export default function NotFound() {
  return (
    <Page>
      <div className="stack center" style={{ '--gap': '18px', alignItems: 'center', paddingTop: 40 } as React.CSSProperties}>
        <motion.div drag dragConstraints={{ left: -120, right: 120, top: -60, bottom: 60 }} whileDrag={{ scale: 1.2, rotate: 20 }} style={{ cursor: 'grab' }}>
          <PillIcon form="capsules" a="var(--coral)" b="var(--lilac)" size={150} />
        </motion.div>
        <h1>404</h1>
        <p className="muted">That shelf is empty. (You can drag the pill.)</p>
        <Link to="/" className="btn coral">
          Home
        </Link>
      </div>
    </Page>
  )
}

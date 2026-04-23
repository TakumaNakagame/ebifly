import { motion, AnimatePresence } from 'framer-motion'

export interface Burst {
  id: string
  emoji: string
  x: number
  y: number
  dx: number
  dy: number
  rot: number
}

interface Props {
  bursts: Burst[]
  onFinish: (id: string) => void
}

export default function Explosion({ bursts, onFinish }: Props) {
  return (
    <AnimatePresence>
      {bursts.map((b) => (
        <motion.div
          key={b.id}
          className="flying-emoji"
          initial={{ left: b.x, top: b.y, opacity: 1, scale: 1, rotate: 0 }}
          animate={{
            left: b.x + b.dx,
            top: b.y + b.dy,
            opacity: 0,
            scale: 2,
            rotate: b.rot,
          }}
          transition={{ duration: 1.2, ease: 'easeOut' }}
          onAnimationComplete={() => onFinish(b.id)}
        >
          {b.emoji}
        </motion.div>
      ))}
    </AnimatePresence>
  )
}

import { motion, AnimatePresence } from 'framer-motion'

export interface Flight {
  id: string
  emoji: string
  targetKey: string
  fromX: number
  fromY: number
  toX: number
  toY: number
  midX: number
  midY: number
  spin: number
}

interface Props {
  flights: Flight[]
  onLand: (flight: Flight) => void
}

// Build dense parabola keyframes so the arc draws a smooth curve rather than
// two line segments meeting at a corner (framer-motion interpolates linearly
// between keyframes).
function buildArc(f: Flight, steps: number) {
  // Apex offset above the straight fromY->toY line at t=0.5
  const H = (f.fromY + f.toY) / 2 - f.midY

  const left: number[] = []
  const top: number[] = []
  const rot: number[] = []
  const scale: number[] = []
  const times: number[] = []

  for (let i = 0; i <= steps; i++) {
    const t = i / steps
    times.push(t)
    left.push(f.fromX + (f.toX - f.fromX) * t)
    const yLinear = f.fromY + (f.toY - f.fromY) * t
    top.push(yLinear - H * 4 * t * (1 - t))
    rot.push(-30 + (f.spin + 30) * t)
    // pop up quickly then settle to 1.0
    const s = t < 0.25 ? 0.4 + (t / 0.25) * 1.1 : 1.5 - ((t - 0.25) / 0.75) * 0.5
    scale.push(s)
  }
  return { left, top, rot, scale, times }
}

export default function FlyingEmoji({ flights, onLand }: Props) {
  return (
    <AnimatePresence>
      {flights.map((f) => {
        const a = buildArc(f, 24)
        return (
          <motion.div
            key={f.id}
            className="flying-emoji"
            initial={{
              left: f.fromX,
              top: f.fromY,
              opacity: 0,
              scale: 0.4,
              rotate: -30,
            }}
            animate={{
              left: a.left,
              top: a.top,
              rotate: a.rot,
              scale: a.scale,
              opacity: [0, 1, 1],
            }}
            exit={{ opacity: 0, scale: 0.3 }}
            transition={{
              duration: 1.1,
              // Smooth parabola across many tiny linear segments.
              left: { times: a.times, ease: 'linear' },
              top: { times: a.times, ease: 'linear' },
              rotate: { times: a.times, ease: 'linear' },
              scale: { times: a.times, ease: 'linear' },
              opacity: { times: [0, 0.1, 1], duration: 1.1 },
            }}
            onAnimationComplete={() => onLand(f)}
          >
            {f.emoji}
          </motion.div>
        )
      })}
    </AnimatePresence>
  )
}

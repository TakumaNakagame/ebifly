import { useRef } from 'react'
import type { Card, Participant, Vote } from '../lib/types'

interface Props {
  p: Participant
  vote: Vote | undefined
  revealedValue: Card | undefined
  phase: 'voting' | 'revealed'
  isMe: boolean
  onKick: () => void
  onThrowTarget: () => void
  accumulated: string[]
  pileRef?: (el: HTMLDivElement | null) => void
}

const CARD_LABEL: Record<string, string> = { coffee: '☕', '0.5': '½' }

export default function ParticipantCard({
  p,
  vote,
  revealedValue,
  phase,
  isMe,
  onKick,
  onThrowTarget,
  accumulated,
  pileRef,
}: Props) {
  const localRef = useRef<HTMLDivElement | null>(null)
  const setRef = (el: HTMLDivElement | null) => {
    localRef.current = el
    pileRef?.(el)
  }

  const status = phase === 'voting'
    ? vote?.isSpectating
      ? '👀 観戦'
      : vote?.hasVoted
        ? '✅ 投票済'
        : '⏳ 考え中…'
    : vote?.isSpectating
      ? '👀 観戦'
      : revealedValue
        ? ''
        : '—'

  return (
    <div className={`pcard ${p.online ? '' : 'offline'}`}>
      <div className="pname" title={p.name}>{p.name}{isMe && ' (あなた)'}</div>

      {phase === 'voting' || vote?.isSpectating || !revealedValue ? (
        <div className={`status ${vote?.isSpectating ? 'spec' : vote?.hasVoted ? 'voted' : ''}`}>
          {status}
        </div>
      ) : (
        <div className="revealed-value">
          {CARD_LABEL[revealedValue] ?? revealedValue}
        </div>
      )}

      <div className="actions">
        <button onClick={onThrowTarget} title="この人に投げる">🎯</button>
        {!isMe && <button onClick={onKick} title="キック">kick</button>}
      </div>

      <div className="emoji-pile" ref={setRef}>
        {accumulated.slice(-8).map((e, i) => (
          <span key={i}>{e}</span>
        ))}
      </div>
    </div>
  )
}

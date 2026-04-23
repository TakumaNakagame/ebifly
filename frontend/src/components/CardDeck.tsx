import type { Card } from '../lib/types'
import { CARDS } from '../lib/types'

interface Props {
  selected: Card | undefined
  isSpectating: boolean
  disabled: boolean
  onPick: (c: Card) => void
  onSpectate: () => void
}

const LABEL: Record<Card, string> = {
  '0': '0',
  '1': '1',
  '2': '2',
  '3': '3',
  '5': '5',
  '8': '8',
  '13': '13',
  '21': '21',
  '?': '?',
  coffee: '☕',
}

export default function CardDeck({ selected, isSpectating, disabled, onPick, onSpectate }: Props) {
  return (
    <div className="deck">
      {CARDS.map((c) => (
        <button
          key={c}
          className={`card ${selected === c && !isSpectating ? 'selected' : ''} ${disabled ? 'disabled' : ''}`}
          disabled={disabled}
          onClick={() => onPick(c)}
        >
          {LABEL[c]}
        </button>
      ))}
      <button
        className={`card spectator ${isSpectating ? 'selected' : ''} ${disabled ? 'disabled' : ''}`}
        disabled={disabled}
        onClick={onSpectate}
      >
        👀 観戦
      </button>
    </div>
  )
}

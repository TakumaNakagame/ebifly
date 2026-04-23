import { useState } from 'react'

const PLACEHOLDER_NAMES = [
  'えびふらい',
  '甘エビ',
  'ぷりぷり',
  '伊勢海老大王',
  'エビマヨ侍',
  'しっぽ',
  'エビフライトライアングル',
]

function pickPlaceholder() {
  return PLACEHOLDER_NAMES[Math.floor(Math.random() * PLACEHOLDER_NAMES.length)]
}

export default function NameModal({
  initial,
  onSubmit,
}: {
  initial: string
  onSubmit: (name: string) => void
}) {
  const [name, setName] = useState(initial)
  const [placeholder] = useState(pickPlaceholder)

  function submit() {
    const n = name.trim().slice(0, 40)
    if (!n) return
    onSubmit(n)
  }
  return (
    <div className="modal-backdrop">
      <div className="modal">
        <h2>お名前を入力</h2>
        <input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
          placeholder={placeholder}
          maxLength={40}
        />
        <button className="primary" onClick={submit} disabled={!name.trim()}>
          部屋に入る
        </button>
      </div>
    </div>
  )
}

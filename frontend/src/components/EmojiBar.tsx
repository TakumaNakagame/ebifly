import { useEffect, useRef, useState } from 'react'
import EmojiPicker, { EmojiStyle, Theme } from 'emoji-picker-react'
import { DEFAULT_FAVORITES, getFavorites, setFavorites } from '../lib/storage'
import type { Participant } from '../lib/types'

interface Props {
  participants: Participant[]
  selfId: string | null
  onThrow: (emoji: string, targetParticipantId?: string) => void
}

const TARGET_ALL = '__all__'

export default function EmojiBar({ participants, selfId, onThrow }: Props) {
  const [favs, setFavs] = useState<string[]>(() => getFavorites())
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerMode, setPickerMode] = useState<'throw' | 'favorite'>('throw')
  const [target, setTarget] = useState<string>(TARGET_ALL)
  const popupRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setFavorites(favs)
  }, [favs])

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (popupRef.current && !popupRef.current.contains(e.target as Node)) {
        setPickerOpen(false)
      }
    }
    if (pickerOpen) {
      document.addEventListener('mousedown', onDocClick)
      return () => document.removeEventListener('mousedown', onDocClick)
    }
  }, [pickerOpen])

  function throwIt(emoji: string) {
    onThrow(emoji, target === TARGET_ALL ? undefined : target)
  }

  function openFavoritePicker() {
    setPickerMode('favorite')
    setPickerOpen(true)
  }
  function openThrowPicker() {
    setPickerMode('throw')
    setPickerOpen(true)
  }

  function onPick(emoji: string) {
    if (pickerMode === 'favorite') {
      setFavs((cur) => {
        if (cur.includes(emoji)) return cur
        return [...cur, emoji].slice(0, 5)
      })
    } else {
      throwIt(emoji)
    }
    setPickerOpen(false)
  }

  function removeFav(i: number) {
    setFavs((cur) => cur.filter((_, idx) => idx !== i))
  }

  const slots: Array<string | null> = [...favs]
  while (slots.length < 5) slots.push(null)

  return (
    <div className="emoji-bar" style={{ position: 'relative' }}>
      <span style={{ fontSize: 13, color: 'var(--muted)' }}>⭐</span>
      <div className="favs">
        {slots.map((e, i) =>
          e ? (
            <div
              key={i}
              className="fav-slot"
              onClick={() => throwIt(e)}
              onContextMenu={(ev) => {
                ev.preventDefault()
                removeFav(i)
              }}
              title="クリックで投擲、右クリックで削除"
            >
              {e}
            </div>
          ) : (
            <div
              key={i}
              className="fav-slot empty"
              onClick={openFavoritePicker}
              title="お気に入りに追加"
            >
              ＋
            </div>
          ),
        )}
      </div>
      <button
        className="picker-button"
        onClick={() => setFavs([...DEFAULT_FAVORITES])}
        title="プリセットに戻す"
      >
        ↻ 初期化
      </button>
      <button className="picker-button" onClick={openThrowPicker}>
        🎯 絵文字を投げる
      </button>
      <span className="target-select">投げ先:</span>
      <select value={target} onChange={(e) => setTarget(e.target.value)}>
        <option value={TARGET_ALL}>全員</option>
        {participants
          .filter((p) => p.id !== selfId)
          .map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
      </select>

      {pickerOpen && (
        <div ref={popupRef} className="picker-popup">
          <EmojiPicker
            onEmojiClick={(data) => onPick(data.emoji)}
            emojiStyle={EmojiStyle.NATIVE}
            theme={Theme.DARK}
            height={350}
            width={320}
          />
        </div>
      )}
    </div>
  )
}

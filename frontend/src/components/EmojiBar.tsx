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
  const [editing, setEditing] = useState(false)
  const [editingSlot, setEditingSlot] = useState<number | null>(null)
  const [target, setTarget] = useState<string>(TARGET_ALL)
  const popupRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setFavorites(favs)
  }, [favs])

  useEffect(() => {
    if (editingSlot === null) return
    function onDocClick(e: MouseEvent) {
      if (popupRef.current && !popupRef.current.contains(e.target as Node)) {
        setEditingSlot(null)
      }
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [editingSlot])

  function toggleEdit() {
    setEditing((prev) => {
      if (prev) setEditingSlot(null)
      return !prev
    })
  }

  function onSlotClick(i: number, value: string | null) {
    if (editing) {
      setEditingSlot(i)
    } else if (value) {
      onThrow(value, target === TARGET_ALL ? undefined : target)
    }
  }

  function onSlotRightClick(e: React.MouseEvent, i: number) {
    e.preventDefault()
    setFavs((cur) => cur.filter((_, idx) => idx !== i))
  }

  function onPickEmoji(emoji: string) {
    if (editingSlot === null) return
    const i = editingSlot
    setFavs((cur) => {
      if (i < cur.length) {
        const next = [...cur]
        next[i] = emoji
        return next
      }
      return [...cur, emoji].slice(0, 5)
    })
    setEditingSlot(null)
  }

  const slots: Array<string | null> = [...favs]
  while (slots.length < 5) slots.push(null)

  return (
    <div className="emoji-bar" style={{ position: 'relative' }}>
      <span style={{ fontSize: 13, color: 'var(--muted)' }}>⭐</span>
      <div className="favs">
        {slots.map((e, i) => {
          const classes = [
            'fav-slot',
            e ? '' : 'empty',
            editing ? 'editing' : '',
            editingSlot === i ? 'picking' : '',
          ].filter(Boolean).join(' ')
          const title = editing
            ? 'クリックで絵文字を選択、右クリックで削除'
            : e
              ? 'クリックで投擲、右クリックで削除'
              : 'プリセット編集モードで設定'
          return (
            <div
              key={i}
              className={classes}
              onClick={() => onSlotClick(i, e)}
              onContextMenu={(ev) => e && onSlotRightClick(ev, i)}
              title={title}
            >
              {e ?? (editing ? '?' : '＋')}
            </div>
          )
        })}
      </div>

      <button
        className={`picker-button ${editing ? 'active' : ''}`}
        onClick={toggleEdit}
        title={editing ? 'プリセット編集を終了' : 'プリセット編集を開始'}
      >
        {editing ? '✓ 完了' : '⚙️ プリセット編集'}
      </button>

      {editing && (
        <button
          className="picker-button"
          onClick={() => {
            setFavs([...DEFAULT_FAVORITES])
            setEditingSlot(null)
          }}
          title="プリセットに戻す"
        >
          ↻ 初期化
        </button>
      )}

      {!editing && (
        <>
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
        </>
      )}

      {editingSlot !== null && (
        <div ref={popupRef} className="picker-popup">
          <EmojiPicker
            onEmojiClick={(data) => onPickEmoji(data.emoji)}
            emojiStyle={EmojiStyle.NATIVE}
            theme={Theme.AUTO}
            height={350}
            width={320}
          />
        </div>
      )}
    </div>
  )
}

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createRoom } from '../lib/api'
import ThemeToggle from '../components/ThemeToggle'

export default function Top() {
  const navigate = useNavigate()
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  async function onCreate() {
    setBusy(true)
    setErr('')
    try {
      const r = await createRoom()
      navigate(`/r/${r.code}`)
    } catch {
      setErr('部屋の作成に失敗しました')
    } finally {
      setBusy(false)
    }
  }

  function onJoin() {
    const c = code.trim().toUpperCase()
    if (c.length === 0) return
    navigate(`/r/${c}`)
  }

  return (
    <div className="top">
      <ThemeToggle />
      <h1 className="brand">
        <span className="brand-main">🦐 ebifly 🦐</span>
        <span className="brand-sub">Planning Poker</span>
      </h1>
      <button className="primary" onClick={onCreate} disabled={busy}>
        ＋ 部屋を作る
      </button>
      <div className="sep">── または ──</div>
      <div className="join">
        <input
          placeholder="ルームコード"
          value={code}
          maxLength={6}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          onKeyDown={(e) => e.key === 'Enter' && onJoin()}
        />
        <button onClick={onJoin}>参加</button>
      </div>
      {err && <p className="err">{err}</p>}
    </div>
  )
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import NameModal from '../components/NameModal'
import ParticipantCard from '../components/ParticipantCard'
import CardDeck from '../components/CardDeck'
import RevealPanel from '../components/RevealPanel'
import EmojiBar from '../components/EmojiBar'
import FlyingEmoji, { type Flight } from '../components/FlyingEmoji'
import Explosion, { type Burst } from '../components/Explosion'
import { joinRoom, checkRoom } from '../lib/api'
import { getStoredName, setStoredName } from '../lib/storage'
import { useRoom } from '../hooks/useRoom'
import type { Card } from '../lib/types'

type JoinStatus = 'loading' | 'need_name' | 'joining' | 'ready' | 'missing' | 'error'

export default function Room() {
  const { code = '' } = useParams()
  const navigate = useNavigate()
  const [status, setStatus] = useState<JoinStatus>('loading')
  const [participantId, setParticipantId] = useState<string | null>(null)
  const [view, actions] = useRoom(code, participantId)

  const [flights, setFlights] = useState<Flight[]>([])
  const [bursts, setBursts] = useState<Burst[]>([])
  const [accumulated, setAccumulated] = useState<Record<string, string[]>>({})
  const pileRefs = useRef<Record<string, HTMLDivElement | null>>({})
  const handledThrowIds = useRef<Set<string>>(new Set())

  // Step 1: check room exists, decide if we need a name
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const r = await checkRoom(code)
        if (cancelled) return
        if (!r.exists) {
          setStatus('missing')
          return
        }
        const name = getStoredName()
        if (!name) {
          setStatus('need_name')
        } else {
          await doJoin(name)
        }
      } catch {
        if (!cancelled) setStatus('error')
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [code])

  async function doJoin(name: string) {
    setStatus('joining')
    try {
      const r = await joinRoom(code, name)
      setStoredName(r.name)
      setParticipantId(r.participantId)
      setStatus('ready')
    } catch {
      setStatus('error')
    }
  }

  // When state includes prior throws (initial sync), seed accumulated
  useEffect(() => {
    if (!view.throws || view.throws.length === 0) return
    const acc: Record<string, string[]> = {}
    for (const t of view.throws) {
      handledThrowIds.current.add(t.id)
      const key = t.targetParticipantId ?? '__all__'
      if (!acc[key]) acc[key] = []
      acc[key].push(t.emoji)
    }
    setAccumulated((cur) => ({ ...cur, ...acc }))
    // Only run on initial state messages; subsequent single-throws handled below
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view.room?.id])

  // Handle new throws: spawn flights
  useEffect(() => {
    for (const t of view.throws) {
      if (handledThrowIds.current.has(t.id)) continue
      handledThrowIds.current.add(t.id)
      spawnFlight(t.id, t.emoji, t.targetParticipantId)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view.throws.length])

  const spawnFlight = useCallback(
    (id: string, emoji: string, targetId?: string) => {
      const targetKey = targetId ?? '__all__'
      let toX = window.innerWidth / 2
      let toY = window.innerHeight / 2
      const el = pileRefs.current[targetKey]
      if (el) {
        const r = el.getBoundingClientRect()
        toX = r.left + r.width / 2 + (Math.random() - 0.5) * 20
        toY = r.top + r.height / 2
      } else if (targetKey === '__all__') {
        toX = window.innerWidth / 2 + (Math.random() - 0.5) * 200
        toY = 300 + (Math.random() - 0.5) * 100
      }
      // random off-screen start (one of four edges)
      const edge = Math.floor(Math.random() * 4)
      let fromX = 0,
        fromY = 0
      switch (edge) {
        case 0:
          fromX = Math.random() * window.innerWidth
          fromY = -60
          break
        case 1:
          fromX = window.innerWidth + 60
          fromY = Math.random() * window.innerHeight
          break
        case 2:
          fromX = Math.random() * window.innerWidth
          fromY = window.innerHeight + 60
          break
        case 3:
          fromX = -60
          fromY = Math.random() * window.innerHeight
          break
      }
      // Fix arc params at spawn time so re-renders don't retarget the animation.
      const midX = (fromX + toX) / 2
      const peakLift = 140 + Math.random() * 80
      const midY = Math.min(fromY, toY) - peakLift
      const spin = 540 + Math.floor(Math.random() * 360)
      setFlights((cur) => [
        ...cur,
        { id, emoji, targetKey, fromX, fromY, toX, toY, midX, midY, spin },
      ])
    },
    [],
  )

  const onFlightLand = useCallback((f: Flight) => {
    setAccumulated((cur) => ({
      ...cur,
      [f.targetKey]: [...(cur[f.targetKey] ?? []), f.emoji],
    }))
    setFlights((cur) => cur.filter((x) => x.id !== f.id))
  }, [])

  // On reveal: burst EVERY thrown emoji — both landed (accumulated) and
  // still-flying ones — so nothing gets missed just because its arc hadn't
  // finished yet when reveal fired.
  const prevPhase = useRef<string | null>(null)
  useEffect(() => {
    if (!view.room) return
    if (prevPhase.current === 'voting' && view.room.phase === 'revealed') {
      const allByKey: Record<string, string[]> = {}
      for (const [key, list] of Object.entries(accumulated)) {
        if (list.length) allByKey[key] = [...list]
      }
      for (const f of flights) {
        if (!allByKey[f.targetKey]) allByKey[f.targetKey] = []
        allByKey[f.targetKey].push(f.emoji)
      }

      const newBursts: Burst[] = []
      for (const [key, list] of Object.entries(allByKey)) {
        const el = pileRefs.current[key]
        let cx = window.innerWidth / 2
        let cy = 300
        if (el) {
          const r = el.getBoundingClientRect()
          cx = r.left + r.width / 2
          cy = r.top + r.height / 2
        }
        for (const emoji of list) {
          const angle = Math.random() * Math.PI * 2
          const speed = 100 + Math.random() * 150
          newBursts.push({
            id: `burst-${Date.now()}-${Math.random()}`,
            emoji,
            x: cx,
            y: cy,
            dx: Math.cos(angle) * speed,
            dy: Math.sin(angle) * speed - 50,
            rot: (Math.random() - 0.5) * 720,
          })
        }
      }
      setBursts(newBursts)
      setAccumulated({})
      setFlights([])
    }
    prevPhase.current = view.room.phase
  }, [view.room, accumulated, flights])

  // Round reset clears handled set
  useEffect(() => {
    if (view.room?.phase === 'voting' && prevPhase.current === 'revealed') {
      handledThrowIds.current = new Set()
      setAccumulated({})
    }
  }, [view.room?.phase])

  // ---- Derived ----
  const me = useMemo(
    () => view.participants.find((p) => p.id === participantId) ?? null,
    [view.participants, participantId],
  )
  const myVote = participantId ? view.votes[participantId] : undefined
  const phase = view.room?.phase ?? 'voting'

  const { voterCount, votedCount } = useMemo(() => {
    let voters = 0
    let voted = 0
    for (const p of view.participants) {
      if (!p.online) continue
      const v = view.votes[p.id]
      if (v?.isSpectating) continue
      // counts as voter if they've voted OR if their default_mode is voter and they haven't acted
      if (v) {
        voters++
        if (v.hasVoted) voted++
      } else {
        if (p.defaultMode === 'voter') voters++
      }
    }
    return { voterCount: voters, votedCount: voted }
  }, [view.participants, view.votes])

  const revealButtonEnabled = phase === 'voting' && votedCount >= 1

  // ---- Render gating ----
  if (status === 'loading' || status === 'joining') {
    return (
      <div className="splash">
        <p>読み込み中…</p>
      </div>
    )
  }
  if (status === 'missing') {
    return (
      <div className="splash">
        <h1>部屋が見つかりません 🦐</h1>
        <p>ルームコード「{code}」の部屋は存在しないか、1 週間以上使われていないため削除されています。</p>
        <div className="actions">
          <button onClick={() => navigate('/')}>TOP へ戻る</button>
        </div>
      </div>
    )
  }
  if (status === 'error') {
    return (
      <div className="splash">
        <h1>接続できませんでした</h1>
        <p>ネットワークか、サーバー側で問題が発生した可能性があります。</p>
        <div className="actions">
          <button onClick={() => location.reload()} className="primary">再読み込み</button>
          <button onClick={() => navigate('/')}>TOP へ戻る</button>
        </div>
      </div>
    )
  }
  if (status === 'need_name') {
    return (
      <NameModal
        initial={getStoredName()}
        onSubmit={(n) => {
          setStoredName(n)
          doJoin(n)
        }}
      />
    )
  }
  if (view.kicked) {
    return (
      <div className="splash">
        <h1>キックされました 👋</h1>
        <p>この部屋の参加者としては記録されていません。</p>
        <div className="actions">
          <button onClick={() => navigate('/')}>TOP へ戻る</button>
        </div>
      </div>
    )
  }

  // ---- Main room UI ----
  return (
    <div className="room">
      {!view.connected && (
        <div className="conn-banner">
          ⚠️ サーバーと切断されました。自動で再接続を試みています…
        </div>
      )}
      <div className="header">
        <div className="code">
          <span>Room:</span>
          <strong>{code}</strong>
          <button
            title="URLをコピー"
            onClick={() => navigator.clipboard.writeText(location.href)}
          >
            🔗
          </button>
        </div>
        <div className="user">
          {me && (
            <button className="name-edit" onClick={() => renameSelf(me.name, actions.setName)}>
              👤 {me.name} ✎
            </button>
          )}
          <button onClick={() => navigate('/')}>🚪 退出</button>
        </div>
      </div>

      <div className="topic-row">
        <label>お題:</label>
        <input
          type="text"
          placeholder="未設定（任意）"
          value={view.room?.topic ?? ''}
          onChange={(e) => actions.setTopic(e.target.value)}
          disabled={phase === 'revealed'}
          maxLength={100}
        />
      </div>

      <div className="progress">
        Round {view.room?.roundNumber ?? 1} ・ 投票 {votedCount}/{voterCount}
        <span className="progress-bar">
          <div style={{ width: `${voterCount ? (votedCount / voterCount) * 100 : 0}%` }} />
        </span>
      </div>

      <div className="participants">
        {view.participants.map((p) => (
          <ParticipantCard
            key={p.id}
            p={p}
            vote={view.votes[p.id]}
            revealedValue={view.revealValues[p.id] as Card | undefined}
            phase={phase}
            isMe={p.id === participantId}
            onKick={() => {
              if (confirm(`${p.name} をキックしますか?`)) actions.kick(p.id)
            }}
            onThrowTarget={() => {
              /* no-op; EmojiBar has target select */
            }}
            accumulated={accumulated[p.id] ?? []}
            pileRef={(el) => {
              pileRefs.current[p.id] = el
            }}
          />
        ))}
      </div>

      {phase === 'voting' && (
        <>
          <CardDeck
            selected={myVote?.value}
            isSpectating={myVote?.isSpectating ?? false}
            disabled={false}
            onPick={actions.vote}
            onSpectate={actions.spectate}
          />
          <div style={{ textAlign: 'center', margin: '16px 0' }}>
            <button
              className="primary"
              onClick={actions.reveal}
              disabled={!revealButtonEnabled}
              title={!revealButtonEnabled ? '投票者が 1 名以上必要' : ''}
            >
              🎬 開票する
            </button>
            <div className="muted" style={{ marginTop: 6 }}>
              {voterCount >= 2 ? '全員投票で自動開票' : '1 名でも投票すればボタンで開票可'}
            </div>
          </div>
        </>
      )}

      {phase === 'revealed' &&
        (view.stats ? (
          <RevealPanel stats={view.stats} onNext={actions.nextRound} />
        ) : (
          <div style={{ textAlign: 'center', margin: '24px 0' }}>
            <button className="primary" onClick={actions.nextRound}>
              ▶ 次のラウンド
            </button>
          </div>
        ))}

      <EmojiBar
        participants={view.participants}
        selfId={participantId}
        onThrow={actions.throwEmoji}
      />

      <FlyingEmoji flights={flights} onLand={onFlightLand} />
      <Explosion
        bursts={bursts}
        onFinish={(id) => setBursts((cur) => cur.filter((b) => b.id !== id))}
      />
    </div>
  )
}

function renameSelf(current: string, setName: (n: string) => void) {
  const n = prompt('新しい名前', current)
  if (n && n.trim()) {
    const v = n.trim().slice(0, 40)
    setStoredName(v)
    setName(v)
  }
}

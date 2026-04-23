import { useCallback, useEffect, useRef, useState } from 'react'
import type {
  Card,
  ClientMsg,
  Participant,
  RoomState,
  ServerMsg,
  Stats,
  Throw,
  Vote,
} from '../lib/types'

export interface RoomView {
  connected: boolean
  room: RoomState | null
  participants: Participant[]
  votes: Record<string, Vote>
  throws: Throw[]
  stats: Stats | null
  revealValues: Record<string, Card>
  kicked: boolean
}

export interface RoomActions {
  vote: (v: Card) => void
  spectate: () => void
  setTopic: (t: string) => void
  setName: (n: string) => void
  reveal: () => void
  nextRound: () => void
  kick: (pid: string) => void
  throwEmoji: (emoji: string, targetParticipantId?: string) => void
}

export function useRoom(code: string, participantId: string | null): [RoomView, RoomActions] {
  const [view, setView] = useState<RoomView>({
    connected: false,
    room: null,
    participants: [],
    votes: {},
    throws: [],
    stats: null,
    revealValues: {},
    kicked: false,
  })
  const wsRef = useRef<WebSocket | null>(null)

  const send = useCallback((msg: ClientMsg) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg))
    }
  }, [])

  useEffect(() => {
    if (!participantId || !code) return

    let cancelled = false
    let attempts = 0
    let reconnectTimer: number | null = null
    let shouldReconnect = true

    const connect = () => {
      if (cancelled) return
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const ws = new WebSocket(`${proto}://${location.host}/ws/${encodeURIComponent(code)}`)
      wsRef.current = ws

      ws.onopen = () => {
        attempts = 0
        setView((v) => ({ ...v, connected: true }))
      }
      ws.onmessage = (ev) => {
        let m: ServerMsg
        try {
          m = JSON.parse(ev.data)
        } catch {
          return
        }
        if (m.type === 'youWereKicked') {
          shouldReconnect = false
        }
        setView((v) => applyMessage(v, m))
      }
      ws.onerror = () => {
        // onclose will fire right after; handle reconnect there.
      }
      ws.onclose = () => {
        setView((v) => ({ ...v, connected: false }))
        if (cancelled || !shouldReconnect) return
        // Exponential backoff: 500ms, 1s, 2s, 4s, ..., capped at 15s.
        const delay = Math.min(15_000, 500 * 2 ** attempts)
        attempts++
        reconnectTimer = window.setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      cancelled = true
      shouldReconnect = false
      if (reconnectTimer !== null) clearTimeout(reconnectTimer)
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [code, participantId])

  const actions: RoomActions = {
    vote: (value) => send({ type: 'vote', value }),
    spectate: () => send({ type: 'spectate' }),
    setTopic: (topic) => send({ type: 'setTopic', topic }),
    setName: (name) => send({ type: 'setName', name }),
    reveal: () => send({ type: 'reveal' }),
    nextRound: () => send({ type: 'nextRound' }),
    kick: (pid) => send({ type: 'kick', participantId: pid }),
    throwEmoji: (emoji, targetParticipantId) =>
      send({ type: 'throwEmoji', emoji, targetParticipantId }),
  }
  return [view, actions]
}

function applyMessage(v: RoomView, m: ServerMsg): RoomView {
  switch (m.type) {
    case 'state': {
      const votes: Record<string, Vote> = {}
      const revealValues: Record<string, Card> = {}
      for (const vv of m.votes) {
        votes[vv.participantId] = vv
        if (vv.value) revealValues[vv.participantId] = vv.value
      }
      return {
        ...v,
        room: m.room,
        participants: m.participants,
        votes,
        throws: m.throws ?? [],
        stats: m.stats ?? null,
        revealValues,
      }
    }
    case 'participantJoined': {
      const others = v.participants.filter((p) => p.id !== m.participant.id)
      return { ...v, participants: [...others, m.participant] }
    }
    case 'participantLeft':
      return {
        ...v,
        participants: v.participants.filter((p) => p.id !== m.participantId),
      }
    case 'participantUpdated': {
      const ps = v.participants.map((p) => (p.id === m.participant.id ? m.participant : p))
      return { ...v, participants: ps }
    }
    case 'participantOnline':
      return {
        ...v,
        participants: v.participants.map((p) => (p.id === m.participantId ? { ...p, online: true } : p)),
      }
    case 'participantOffline':
      return {
        ...v,
        participants: v.participants.map((p) =>
          p.id === m.participantId ? { ...p, online: false } : p,
        ),
      }
    case 'voteUpdated': {
      const existing = v.votes[m.participantId]
      return {
        ...v,
        votes: {
          ...v.votes,
          [m.participantId]: {
            participantId: m.participantId,
            hasVoted: m.hasVoted,
            isSpectating: m.isSpectating,
            value: existing?.value,
          },
        },
      }
    }
    case 'topicChanged':
      return v.room ? { ...v, room: { ...v.room, topic: m.topic } } : v
    case 'revealed': {
      const rv: Record<string, Card> = {}
      for (const vv of m.votes) rv[vv.participantId] = vv.value
      const room = v.room ? { ...v.room, phase: 'revealed' as const } : null
      return { ...v, room, stats: m.stats, revealValues: rv, throws: [] }
    }
    case 'roundStarted': {
      const room = v.room ? { ...v.room, phase: 'voting' as const, roundNumber: m.roundNumber, topic: '' } : null
      return { ...v, room, votes: {}, stats: null, revealValues: {}, throws: [] }
    }
    case 'emojiThrown':
      return {
        ...v,
        throws: [
          ...v.throws,
          {
            id: m.throwId,
            emoji: m.emoji,
            targetParticipantId: m.targetParticipantId,
            thrownAt: Date.now(),
          },
        ],
      }
    case 'kicked':
      return {
        ...v,
        participants: v.participants.filter((p) => p.id !== m.participantId),
      }
    case 'youWereKicked':
      return { ...v, kicked: true }
    default:
      return v
  }
}

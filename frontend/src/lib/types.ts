export type Phase = 'voting' | 'revealed'
export type Mode = 'voter' | 'spectator'

export const CARDS = ['0', '0.5', '1', '2', '3', '5', '8', '13', '21', '?', 'coffee'] as const
export type Card = (typeof CARDS)[number]

export interface RoomState {
  id: string
  code: string
  topic: string
  phase: Phase
  roundNumber: number
}

export interface Participant {
  id: string
  name: string
  defaultMode: Mode
  online: boolean
}

export interface Vote {
  participantId: string
  hasVoted: boolean
  isSpectating: boolean
  value?: Card
}

export interface Throw {
  id: string
  emoji: string
  targetParticipantId?: string
  thrownAt: number
}

export interface Stats {
  average: number | null
  distribution: Record<string, number>
}

export type ServerMsg =
  | { type: 'state'; room: RoomState; participants: Participant[]; votes: Vote[]; throws: Throw[]; stats?: Stats | null }
  | { type: 'participantJoined'; participant: Participant }
  | { type: 'participantLeft'; participantId: string }
  | { type: 'participantUpdated'; participant: Participant }
  | { type: 'participantOnline'; participantId: string }
  | { type: 'participantOffline'; participantId: string }
  | { type: 'voteUpdated'; participantId: string; hasVoted: boolean; isSpectating: boolean }
  | { type: 'topicChanged'; topic: string }
  | { type: 'revealed'; votes: { participantId: string; value: Card }[]; stats: Stats }
  | { type: 'roundStarted'; roundNumber: number }
  | { type: 'emojiThrown'; emoji: string; targetParticipantId?: string; throwId: string }
  | { type: 'kicked'; participantId: string }
  | { type: 'youWereKicked' }
  | { type: 'error'; code: string; message: string }
  | { type: 'pong' }

export type ClientMsg =
  | { type: 'vote'; value: Card }
  | { type: 'spectate' }
  | { type: 'setTopic'; topic: string }
  | { type: 'setName'; name: string }
  | { type: 'reveal' }
  | { type: 'nextRound' }
  | { type: 'kick'; participantId: string }
  | { type: 'throwEmoji'; emoji: string; targetParticipantId?: string }
  | { type: 'ping' }

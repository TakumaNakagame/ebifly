import { isGranted } from './consent'

const NAME_KEY = 'pp.name'
const FAVORITES_KEY = 'pp.favoriteEmojis'

// Fun-to-throw defaults for first-time users.
export const DEFAULT_FAVORITES = ['🦐', '🍣', '💣', '🎉', '🍕']

// All getters/setters here are gated by consent. Without consent we only
// return defaults on read and silently skip writes — the app keeps working
// through in-memory React state, just without persistence across reloads.

export function getStoredName(): string {
  if (!isGranted()) return ''
  return localStorage.getItem(NAME_KEY) ?? ''
}
export function setStoredName(name: string) {
  if (!isGranted()) return
  localStorage.setItem(NAME_KEY, name)
}

export function getFavorites(): string[] {
  if (!isGranted()) return [...DEFAULT_FAVORITES]
  try {
    const raw = localStorage.getItem(FAVORITES_KEY)
    if (raw === null) return [...DEFAULT_FAVORITES]
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return [...DEFAULT_FAVORITES]
    const cleaned = parsed.filter((v) => typeof v === 'string').slice(0, 5)
    if (cleaned.length === 0) return [...DEFAULT_FAVORITES]
    return cleaned
  } catch {
    return [...DEFAULT_FAVORITES]
  }
}

export function setFavorites(list: string[]) {
  if (!isGranted()) return
  localStorage.setItem(FAVORITES_KEY, JSON.stringify(list.slice(0, 5)))
}

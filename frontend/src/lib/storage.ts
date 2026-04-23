const NAME_KEY = 'pp.name'
const FAVORITES_KEY = 'pp.favoriteEmojis'

// Fun-to-throw defaults for first-time users.
export const DEFAULT_FAVORITES = ['🦐', '🍣', '💣', '🎉', '🍕']

export function getStoredName(): string {
  return localStorage.getItem(NAME_KEY) ?? ''
}
export function setStoredName(name: string) {
  localStorage.setItem(NAME_KEY, name)
}

export function getFavorites(): string[] {
  try {
    const raw = localStorage.getItem(FAVORITES_KEY)
    if (raw === null) return [...DEFAULT_FAVORITES]
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return [...DEFAULT_FAVORITES]
    const cleaned = parsed.filter((v) => typeof v === 'string').slice(0, 5)
    // If the user has no favorites (e.g. initial state previously persisted as
    // an empty array), seed the defaults so they're not stuck with empty slots.
    if (cleaned.length === 0) return [...DEFAULT_FAVORITES]
    return cleaned
  } catch {
    return [...DEFAULT_FAVORITES]
  }
}

export function setFavorites(list: string[]) {
  localStorage.setItem(FAVORITES_KEY, JSON.stringify(list.slice(0, 5)))
}

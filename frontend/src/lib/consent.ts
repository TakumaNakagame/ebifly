// Consent management for non-essential browser storage.
//
// Essential cookies (participant ID) are set regardless of consent, because
// they are required for service functionality. All other persistence
// (pp.name / pp.favoriteEmojis / pp.theme) is gated by user consent.

const KEY = 'pp.consent'
const EVENT = 'pp-consent-changed'
const PREFERENCE_KEYS = ['pp.name', 'pp.favoriteEmojis', 'pp.theme']

export type Consent = 'granted' | 'denied' | null

export function getConsent(): Consent {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'granted' || v === 'denied') return v
  } catch {
    /* private mode / disabled */
  }
  return null
}

export function setConsent(v: 'granted' | 'denied') {
  localStorage.setItem(KEY, v)
  if (v === 'denied') {
    // Revoke: wipe anything we previously persisted.
    for (const k of PREFERENCE_KEYS) localStorage.removeItem(k)
  }
  emit()
}

/** Reset to "undecided" so the banner reappears and the user can re-choose. */
export function resetConsent() {
  localStorage.removeItem(KEY)
  for (const k of PREFERENCE_KEYS) localStorage.removeItem(k)
  emit()
}

export function isGranted(): boolean {
  return getConsent() === 'granted'
}

function emit() {
  window.dispatchEvent(new Event(EVENT))
}

export function onConsentChange(fn: () => void): () => void {
  window.addEventListener(EVENT, fn)
  return () => window.removeEventListener(EVENT, fn)
}

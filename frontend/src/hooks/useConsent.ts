import { useEffect, useState } from 'react'
import type { Consent } from '../lib/consent'
import { getConsent, onConsentChange, resetConsent, setConsent } from '../lib/consent'

export function useConsent() {
  const [consent, setLocal] = useState<Consent>(() => getConsent())

  useEffect(() => onConsentChange(() => setLocal(getConsent())), [])

  return {
    consent,
    grant: () => setConsent('granted'),
    deny: () => setConsent('denied'),
    reset: () => resetConsent(),
  }
}

import { useCallback, useEffect, useState } from 'react'
import { isGranted } from '../lib/consent'

export type ThemeChoice = 'auto' | 'light' | 'dark'

const KEY = 'pp.theme'

function read(): ThemeChoice {
  if (!isGranted()) return 'auto'
  const v = localStorage.getItem(KEY)
  return v === 'light' || v === 'dark' ? v : 'auto'
}

function apply(choice: ThemeChoice) {
  const root = document.documentElement
  if (choice === 'auto') {
    root.removeAttribute('data-theme')
  } else {
    root.setAttribute('data-theme', choice)
  }
}

export function useTheme() {
  const [theme, setThemeState] = useState<ThemeChoice>(() => {
    const t = read()
    apply(t)
    return t
  })

  const setTheme = useCallback((next: ThemeChoice) => {
    setThemeState(next)
    apply(next)
    if (!isGranted()) return
    if (next === 'auto') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, next)
  }, [])

  const cycle = useCallback(() => {
    setTheme(theme === 'auto' ? 'light' : theme === 'light' ? 'dark' : 'auto')
  }, [theme, setTheme])

  // Keep the CSS in sync if the OS flips while the tab is open and we're in auto.
  useEffect(() => {
    if (theme !== 'auto') return
    const mq = window.matchMedia('(prefers-color-scheme: light)')
    const handler = () => apply('auto')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [theme])

  return { theme, setTheme, cycle }
}

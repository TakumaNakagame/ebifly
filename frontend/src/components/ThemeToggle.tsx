import { useTheme } from '../hooks/useTheme'

const LABELS = {
  auto: { icon: '🌓', text: 'Auto' },
  light: { icon: '☀️', text: 'Light' },
  dark: { icon: '🌙', text: 'Dark' },
} as const

export default function ThemeToggle() {
  const { theme, cycle } = useTheme()
  const { icon, text } = LABELS[theme]
  return (
    <button
      className="theme-toggle"
      onClick={cycle}
      title={`Theme: ${text} (click to cycle)`}
      aria-label={`Switch theme (current: ${text})`}
    >
      <span className="theme-icon">{icon}</span>
      <span className="theme-text">{text}</span>
    </button>
  )
}

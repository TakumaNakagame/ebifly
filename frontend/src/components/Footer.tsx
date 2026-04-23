import { useConsent } from '../hooks/useConsent'

export default function Footer() {
  const { consent, reset } = useConsent()
  const statusLabel =
    consent === 'granted' ? '同意済' : consent === 'denied' ? '拒否中' : '未選択'
  return (
    <footer className="page-footer">
      <a
        href="https://x.com/kameneko1004"
        target="_blank"
        rel="noopener noreferrer"
        className="credit"
      >
        <img
          src="https://avatars.githubusercontent.com/u/5129906?v=4"
          alt="kameneko"
          className="avatar"
          referrerPolicy="no-referrer"
        />
        <span>Created by kameneko</span>
      </a>
      <span className="sep">·</span>
      <a
        href="https://github.com/TakumaNakagame/ebifly"
        target="_blank"
        rel="noopener noreferrer"
      >
        GitHub ↗
      </a>
      <span className="sep">·</span>
      <button
        className="privacy-link"
        onClick={reset}
        title="プライバシー設定を開いて同意状態を変更"
      >
        プライバシー設定 <span className="consent-status">({statusLabel})</span>
      </button>
    </footer>
  )
}

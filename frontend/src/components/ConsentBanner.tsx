import { useState } from 'react'
import { useConsent } from '../hooks/useConsent'

export default function ConsentBanner() {
  const { consent, grant, deny } = useConsent()
  const [thanking, setThanking] = useState(false)

  function onGrant() {
    grant()
    setThanking(true)
    setTimeout(() => setThanking(false), 5000)
  }

  if (consent !== null && !thanking) return null

  if (thanking) {
    return (
      <div className="consent-banner consent-thanks" role="status">
        <div className="thanks-content">
          <div className="thanks-shrimp">🦐</div>
          <p className="thanks-title">ありがとう！！</p>
          <p className="thanks-detail">
            次回以降、お名前・お気に入り・テーマを覚えておきます 🎉
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="consent-banner" role="dialog" aria-label="ブラウザ保存の同意">
      <div className="consent-body">
        <p className="consent-head">🦐 ブラウザへの保存について</p>
        <p className="consent-detail">
          ebifly は、<strong>お名前・お気に入り絵文字・テーマ選好</strong>
          をブラウザに保存すると次回以降の利便性が向上します。保存に同意しますか？
        </p>
        <p className="consent-note">
          なお、部屋参加時の参加者識別 Cookie（<code>pp_pid_XXX</code>
          ）はサービス動作に必須なため、同意の有無にかかわらず発行されます。
          同意はフッターの「プライバシー設定」からいつでも変更できます。
        </p>
      </div>
      <div className="consent-actions">
        <button onClick={deny}>拒否</button>
        <button className="primary" onClick={onGrant}>
          同意する
        </button>
      </div>
    </div>
  )
}

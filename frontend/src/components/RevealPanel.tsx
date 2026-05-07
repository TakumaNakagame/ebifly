import type { Stats } from '../lib/types'

export default function RevealPanel({ stats, onNext }: { stats: Stats; onNext: () => void }) {
  const entries = Object.entries(stats.distribution ?? {}).sort((a, b) => {
    const na = parseFloat(a[0])
    const nb = parseFloat(b[0])
    if (!isNaN(na) && !isNaN(nb)) return na - nb
    if (!isNaN(na)) return -1
    if (!isNaN(nb)) return 1
    return a[0].localeCompare(b[0])
  })
  const max = entries.reduce((m, [, c]) => Math.max(m, c), 0) || 1
  return (
    <div className="reveal-panel">
      <h3>📊 集計</h3>
      <div className="stat">
        <div className="muted">平均値</div>
        <div className="avg">{stats.average !== null ? stats.average.toFixed(2) : '—'}</div>
      </div>
      <div className="stat">
        <div className="muted">分布</div>
        {entries.length === 0 ? (
          <div className="muted">—</div>
        ) : (
          entries.map(([label, count]) => (
            <div className="dist-row" key={label}>
              <span className="label">{label === 'coffee' ? '☕' : label === '0.5' ? '½' : label}</span>
              <span className="bar" style={{ width: `${(count / max) * 200}px` }} />
              <span className="count">{count} 人</span>
            </div>
          ))
        )}
      </div>
      <div style={{ textAlign: 'center', marginTop: 20 }}>
        <button className="primary" onClick={onNext}>▶ 次のラウンド</button>
      </div>
    </div>
  )
}

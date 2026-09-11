import type { CheckResult } from './api'

// Latency over the last N checks, drawn as a path rather than bars: at 60px
// wide a bar chart becomes noise, whereas a line still shows shape. Failed
// checks are marked as points on the baseline, so an outage reads as a gap in
// the trace rather than disappearing from the picture entirely.
export default function Sparkline({
  results,
  width = 96,
  height = 24,
}: {
  results: CheckResult[]
  width?: number
  height?: number
}) {
  if (results.length < 2) {
    return <span className="metric">—</span>
  }

  const pad = 2
  const max = Math.max(...results.map((r) => r.latency_ms ?? 0), 1)
  const step = (width - pad * 2) / (results.length - 1)

  const y = (ms: number) => height - pad - (ms / max) * (height - pad * 2)

  const points = results.map((r, i) => ({
    x: pad + i * step,
    y: y(r.latency_ms ?? 0),
    ok: r.ok,
  }))

  const d = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
  const area = `${d} L${points[points.length - 1].x.toFixed(1)} ${height} L${points[0].x.toFixed(1)} ${height} Z`

  const anyFailed = points.some((p) => !p.ok)
  const stroke = anyFailed ? 'var(--warn)' : 'var(--up)'

  return (
    <svg width={width} height={height} aria-hidden style={{ display: 'block' }}>
      <path d={area} fill={stroke} opacity="0.1" />
      <path d={d} fill="none" stroke={stroke} strokeWidth="1.5" strokeLinejoin="round" />
      {points
        .filter((p) => !p.ok)
        .map((p, i) => (
          <circle key={i} cx={p.x} cy={height - pad} r="2" fill="var(--down)" />
        ))}
    </svg>
  )
}

import { useState } from 'react'
import type { SeriesPoint } from './api'

// Hand-drawn SVG rather than a charting library. The shapes needed here are a
// band and two lines; a dependency would add 90 KB to render four paths, and
// would fight the token system on colour.
//
// p50 and p95 together rather than an average: the mean hides the slow tail,
// and the tail is what users actually feel.
export default function LatencyChart({
  series,
  height = 200,
}: {
  series: SeriesPoint[]
  height?: number
}) {
  const [hover, setHover] = useState<number | null>(null)

  const withData = series.filter((p) => p.latency_p50 != null)
  if (withData.length < 2) {
    return (
      <div className="chart-empty" style={{ height }}>
        Not enough history yet. Come back after a few more checks.
      </div>
    )
  }

  const w = 900
  const padL = 44
  const padR = 8
  const padT = 12
  const padB = 24

  const max = Math.max(...withData.map((p) => p.latency_p95 ?? p.latency_p50 ?? 0)) * 1.15
  const step = (w - padL - padR) / (withData.length - 1)

  const x = (i: number) => padL + i * step
  const y = (ms: number) => padT + (1 - ms / max) * (height - padT - padB)

  const line = (pick: (p: SeriesPoint) => number | null) =>
    withData
      .map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)} ${y(pick(p) ?? 0).toFixed(1)}`)
      .join(' ')

  const p50 = line((p) => p.latency_p50)
  const p95 = line((p) => p.latency_p95)

  // The band between p50 and p95 is the spread: a wide band means inconsistent
  // response times even when the median looks fine.
  const band =
    withData.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)} ${y(p.latency_p95 ?? 0).toFixed(1)}`).join(' ') +
    ' ' +
    [...withData]
      .reverse()
      .map((p, i) => `L${x(withData.length - 1 - i).toFixed(1)} ${y(p.latency_p50 ?? 0).toFixed(1)}`)
      .join(' ') +
    ' Z'

  const ticks = [0, 0.5, 1].map((f) => Math.round(max * f))
  const hovered = hover != null ? withData[hover] : null

  return (
    <div className="chart">
      <svg
        viewBox={`0 0 ${w} ${height}`}
        width="100%"
        height={height}
        onMouseLeave={() => setHover(null)}
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect()
          const px = ((e.clientX - rect.left) / rect.width) * w
          const i = Math.round((px - padL) / step)
          setHover(i >= 0 && i < withData.length ? i : null)
        }}
      >
        {ticks.map((t) => (
          <g key={t}>
            <line
              x1={padL}
              x2={w - padR}
              y1={y(t)}
              y2={y(t)}
              stroke="var(--line-soft)"
              strokeWidth="1"
            />
            <text x={padL - 8} y={y(t) + 4} textAnchor="end" className="chart-tick">
              {t}
            </text>
          </g>
        ))}

        <path d={band} fill="var(--accent)" opacity="0.10" />
        <path d={p95} fill="none" stroke="var(--accent)" strokeWidth="1" opacity="0.45" />
        <path d={p50} fill="none" stroke="var(--accent)" strokeWidth="1.75" strokeLinejoin="round" />

        {/* Buckets where a check failed, marked on the axis rather than in the
            line: a failure has no latency, and inventing one would lie. */}
        {withData.map((p, i) =>
          p.uptime < 100 ? (
            <rect
              key={i}
              x={x(i) - step / 2}
              y={padT}
              width={step}
              height={height - padT - padB}
              fill="var(--down)"
              opacity={p.uptime === 0 ? 0.16 : 0.08}
            />
          ) : null,
        )}

        {hovered && (
          <line
            x1={x(hover!)}
            x2={x(hover!)}
            y1={padT}
            y2={height - padB}
            stroke="var(--muted)"
            strokeWidth="1"
            strokeDasharray="3 3"
          />
        )}
      </svg>

      <div className="chart-foot">
        {hovered ? (
          <>
            <span className="num">{new Date(hovered.at).toLocaleString()}</span>
            <span className="chart-key">
              <i style={{ background: 'var(--accent)' }} /> p50{' '}
              <b className="num">{hovered.latency_p50}ms</b>
            </span>
            <span className="chart-key">
              <i style={{ background: 'var(--accent)', opacity: 0.45 }} /> p95{' '}
              <b className="num">{hovered.latency_p95}ms</b>
            </span>
            {hovered.uptime < 100 && (
              <span className="chart-key" style={{ color: 'var(--down)' }}>
                {hovered.uptime.toFixed(0)}% of checks passed
              </span>
            )}
          </>
        ) : (
          <>
            <span className="chart-key">
              <i style={{ background: 'var(--accent)' }} /> median
            </span>
            <span className="chart-key">
              <i style={{ background: 'var(--accent)', opacity: 0.45 }} /> 95th percentile
            </span>
            <span className="metric">Hover for a point in time</span>
          </>
        )}
      </div>
    </div>
  )
}

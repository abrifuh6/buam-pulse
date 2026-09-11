import { useState } from 'react'
import type { SeriesPoint } from './api'
import { useTicker } from './useTicker'

// Hand-drawn SVG rather than a charting library. The shapes needed are a band
// and two lines; a dependency would add 90 KB to render four paths and would
// fight the token system on colour.
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
  // Clicking pins a bucket. Hover is for scanning; pinning is for reading the
  // numbers properly, or pointing at them while someone else looks.
  const [pinned, setPinned] = useState<number | null>(null)
  const now = useTicker()

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
    withData
      .map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)} ${y(p.latency_p95 ?? 0).toFixed(1)}`)
      .join(' ') +
    ' ' +
    [...withData]
      .reverse()
      .map((p, i) => `L${x(withData.length - 1 - i).toFixed(1)} ${y(p.latency_p50 ?? 0).toFixed(1)}`)
      .join(' ') +
    ' Z'

  const ticks = [0, 0.5, 1].map((f) => Math.round(max * f))

  // 72 bpm — a resting heart rate, which is where the product gets its name.
  const beat = (Math.sin((now / 1000) * Math.PI * 2 * (72 / 60)) + 1) / 2
  // The sweep takes four seconds to cross, slow enough to read as deliberate
  // rather than restless.
  const sweepAt = (now / 4000) % 1

  const activeIndex = pinned ?? hover
  const active = activeIndex != null ? withData[activeIndex] : null
  const last = withData[withData.length - 1]

  return (
    <div className="chart">
      <svg
        viewBox={`0 0 ${w} ${height}`}
        width="100%"
        height={height}
        onMouseLeave={() => setHover(null)}
        onClick={() => setPinned(hover === pinned ? null : hover)}
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect()
          const px = ((e.clientX - rect.left) / rect.width) * w
          const i = Math.round((px - padL) / step)
          setHover(i >= 0 && i < withData.length ? i : null)
        }}
      >
        {/* A sweep travels along the trace, the way a monitor's own beam does.
            It carries no data — its job is to say the chart is live rather than
            a screenshot — so it stays faint enough to ignore while reading. */}
        <defs>
          <linearGradient id="sweep" x1="0" x2="1" y1="0" y2="0">
            <stop
              offset={`${Math.max(0, sweepAt - 0.08) * 100}%`}
              stopColor="var(--accent)"
              stopOpacity="0"
            />
            <stop offset={`${sweepAt * 100}%`} stopColor="var(--accent)" stopOpacity="0.55" />
            <stop
              offset={`${Math.min(1, sweepAt + 0.08) * 100}%`}
              stopColor="var(--accent)"
              stopOpacity="0"
            />
          </linearGradient>
        </defs>

        {ticks.map((t) => (
          <g key={t}>
            <line x1={padL} x2={w - padR} y1={y(t)} y2={y(t)} stroke="var(--line-soft)" strokeWidth="1" />
            <text x={padL - 8} y={y(t) + 4} textAnchor="end" className="chart-tick">
              {t}
            </text>
          </g>
        ))}

        {/* Buckets where a check failed, shaded on the axis rather than drawn
            into the line: a failure has no latency, and inventing one lies. */}
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

        <path d={band} fill="var(--accent)" opacity="0.10" />
        <path d={p95} fill="none" stroke="var(--accent)" strokeWidth="1" opacity="0.45" />
        <path d={p50} fill="none" stroke="var(--accent)" strokeWidth="1.75" strokeLinejoin="round" />
        <path d={p50} fill="none" stroke="url(#sweep)" strokeWidth="3.5" strokeLinejoin="round" />

        {/* The most recent point beats, so the right edge is visibly now. */}
        <circle
          cx={x(withData.length - 1)}
          cy={y(last.latency_p50 ?? 0)}
          r={3 + beat * 3}
          fill="var(--accent)"
          opacity={0.12 + beat * 0.22}
        />
        <circle cx={x(withData.length - 1)} cy={y(last.latency_p50 ?? 0)} r="3" fill="var(--accent)" />

        {active && activeIndex != null && (
          <>
            <line
              x1={x(activeIndex)}
              x2={x(activeIndex)}
              y1={padT}
              y2={height - padB}
              stroke={pinned != null ? 'var(--accent)' : 'var(--muted)'}
              strokeWidth="1"
              strokeDasharray={pinned != null ? undefined : '3 3'}
            />
            <circle
              cx={x(activeIndex)}
              cy={y(active.latency_p50 ?? 0)}
              r="3.5"
              fill="var(--accent)"
              stroke="var(--ink-800)"
              strokeWidth="2"
            />
          </>
        )}
      </svg>

      <div className="chart-foot">
        {active ? (
          <>
            <span className="num">{new Date(active.at).toLocaleString()}</span>
            <span className="chart-key">
              <b className="num">{active.checks}</b> checks
            </span>
            <span className="chart-key">
              <i style={{ background: 'var(--accent)' }} /> p50{' '}
              <b className="num">{active.latency_p50}ms</b>
            </span>
            <span className="chart-key">
              <i style={{ background: 'var(--accent)', opacity: 0.45 }} /> p95{' '}
              <b className="num">{active.latency_p95}ms</b>
            </span>
            <span
              className="chart-key"
              style={{ color: active.uptime < 100 ? 'var(--down)' : 'var(--up)' }}
            >
              {active.uptime.toFixed(1)}% passed
            </span>
            {pinned != null && (
              <button className="quiet" onClick={() => setPinned(null)}>
                Unpin
              </button>
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
            <span className="metric">Hover to inspect, click to pin</span>
          </>
        )}
      </div>
    </div>
  )
}

import { useEffect, useRef, useState } from 'react'

// The hero is the product working, not a picture of it. A simulated account
// runs on a loop: one service fails, an alert fires, it recovers. Everything a
// visitor needs to understand Pulse happens in twenty seconds without them
// clicking anything.
//
// The data is fabricated and says so in the caption — claiming these are real
// customers would be a lie, and the demo is more persuasive when it is honest
// about being a demo.

type Service = {
  name: string
  host: string
  base: number // typical latency
  jitter: number
}

const SERVICES: Service[] = [
  { name: 'Checkout API', host: 'api.northgate.co', base: 120, jitter: 40 },
  { name: 'Marketing site', host: 'northgate.co', base: 210, jitter: 70 },
  { name: 'Image CDN', host: 'cdn.northgate.co', base: 64, jitter: 20 },
  { name: 'Postgres primary', host: 'db.northgate.co:5432', base: 8, jitter: 4 },
]

// The third service is the one that breaks, on a loop.
const BREAKS = 2
const CYCLE = 22000 // ms for a full healthy → failing → recovered loop

type Point = { ms: number; ok: boolean }

export default function HeroDemo() {
  const [history, setHistory] = useState<Point[][]>(() =>
    SERVICES.map((s) =>
      Array.from({ length: 40 }, () => ({
        ms: s.base + Math.random() * s.jitter,
        ok: true,
      })),
    ),
  )
  const [phase, setPhase] = useState<'ok' | 'failing' | 'recovered'>('ok')
  const start = useRef(Date.now())

  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return

    const tick = window.setInterval(() => {
      const elapsed = (Date.now() - start.current) % CYCLE
      const failing = elapsed > 9000 && elapsed < 16000
      const recovered = elapsed >= 16000 && elapsed < 19000
      setPhase(failing ? 'failing' : recovered ? 'recovered' : 'ok')

      setHistory((prev) =>
        prev.map((points, i) => {
          const s = SERVICES[i]
          const broken = i === BREAKS && failing
          const next: Point = broken
            ? { ms: 0, ok: false }
            : { ms: s.base + Math.random() * s.jitter, ok: true }
          return [...points.slice(1), next]
        }),
      )
    }, 700)

    return () => window.clearInterval(tick)
  }, [])

  const anyDown = history.some((points, i) => i === BREAKS && !points[points.length - 1].ok)

  return (
    <div className="demo" aria-label="Example Pulse dashboard">
      <div className={`demo-bar ${anyDown ? 'is-down' : ''}`}>
        <div>
          <strong>
            {anyDown ? '1 service is down' : 'All systems operational'}
          </strong>
          <span className="dim">
            {anyDown
              ? 'Image CDN stopped responding · alerting the on-call channel'
              : phase === 'recovered'
                ? 'Image CDN recovered after 1m 12s · everyone notified'
                : '4 services · checked every 30 seconds'}
          </span>
        </div>
        <div className="demo-stats">
          <span className="num">4</span>
          <span className="dim">monitored</span>
          <span className="num" style={{ color: anyDown ? 'var(--down)' : undefined }}>
            {anyDown ? 1 : 0}
          </span>
          <span className="dim">down</span>
        </div>
      </div>

      <table className="demo-table">
        <tbody>
          {SERVICES.map((s, i) => {
            const points = history[i]
            const latest = points[points.length - 1]
            return (
              <tr key={s.name}>
                <td style={{ width: 24 }}>
                  <span className={`dot ${latest.ok ? 'up' : 'down'}`} />
                </td>
                <td>
                  <div className="demo-name">{s.name}</div>
                  <div className="demo-host">{s.host}</div>
                </td>
                <td className="num demo-latency">
                  {latest.ok ? `${Math.round(latest.ms)}ms` : 'timeout'}
                </td>
                <td style={{ width: 130 }}>
                  <Trace points={points} />
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>

      <div className="demo-foot dim">
        Simulated data, running live. The real thing looks exactly like this.
      </div>
    </div>
  )
}

function Trace({ points }: { points: Point[] }) {
  const w = 120
  const h = 26
  const max = Math.max(...points.map((p) => p.ms), 1)
  const step = w / (points.length - 1)
  const y = (ms: number) => h - 2 - (ms / max) * (h - 6)

  const d = points
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)} ${y(p.ms).toFixed(1)}`)
    .join(' ')

  const failing = !points[points.length - 1].ok
  const colour = failing ? 'var(--down)' : 'var(--up)'

  return (
    <svg width={w} height={h} aria-hidden>
      <path d={`${d} L${w} ${h} L0 ${h} Z`} fill={colour} opacity="0.09" />
      <path d={d} fill="none" stroke={colour} strokeWidth="1.5" strokeLinejoin="round" />
      <circle cx={w - step} cy={y(points[points.length - 1].ms)} r="2.5" fill={colour}>
        <animate attributeName="opacity" values="1;0.3;1" dur="1.4s" repeatCount="indefinite" />
      </circle>
    </svg>
  )
}

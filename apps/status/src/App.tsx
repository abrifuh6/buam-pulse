import { useEffect, useState } from 'react'

type Monitor = {
  name: string
  status: 'up' | 'down' | 'unknown'
  uptime_90d: number
  avg_latency_ms: number | null
  daily_uptime: number[]
}
type Incident = {
  monitor: string
  started_at: string
  resolved_at: string | null
  minutes: number
}
type Status = {
  tenant: string
  monitors: Monitor[]
  active_incidents: Incident[]
  recent_incidents: Incident[]
  updated_at: string
}

function slugFromPath(): string {
  return window.location.pathname.replace(/^\/+|\/+$/g, '')
}

function fmtDuration(mins: number): string {
  if (mins < 60) return `${mins}m`
  const h = Math.floor(mins / 60)
  const m = mins % 60
  if (h < 24) return m ? `${h}h ${m}m` : `${h}h`
  return `${Math.floor(h / 24)}d ${h % 24}h`
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

// The 90-day bar. A day with no data is rendered grey rather than dropped, so
// the page stays honest about how much history actually exists.
function UptimeBar({ daily }: { daily: number[] }) {
  return (
    <div className="bar" title="Last 90 days">
      {daily.map((v, i) => (
        <i
          key={i}
          className={v < 0 ? 'nodata' : v >= 99.9 ? 'ok' : v >= 95 ? 'warn' : 'bad'}
          title={v < 0 ? 'no data' : `${v.toFixed(2)}%`}
        />
      ))}
    </div>
  )
}

export default function App() {
  const [data, setData] = useState<Status | null>(null)
  const [err, setErr] = useState('')
  const slug = slugFromPath()

  useEffect(() => {
    if (!slug) {
      setErr('No status page specified.')
      return
    }
    const load = () =>
      fetch(`/api/v1/public/status/${encodeURIComponent(slug)}`)
        .then((r) => {
          if (r.status === 404) throw new Error('Status page not found.')
          if (!r.ok) throw new Error('Unable to load status.')
          return r.json()
        })
        .then(setData)
        .catch((e) => setErr(e.message))

    load()
    const t = window.setInterval(load, 60000)
    return () => window.clearInterval(t)
  }, [slug])

  if (err) return <div className="wrap"><p className="center">{err}</p></div>
  if (!data) return <div className="wrap"><p className="center">Loading…</p></div>

  const banner = data.active_incidents.length
    ? {
        cls: 'bad',
        text: `${data.active_incidents.length} service${
          data.active_incidents.length > 1 ? 's' : ''
        } currently down`,
      }
    : data.monitors.some((m) => m.status === 'up')
      ? { cls: 'ok', text: 'All systems operational' }
      : { cls: 'idle', text: 'Awaiting first checks' }

  return (
    <div className="wrap">
      <h1>{data.tenant}</h1>
      <p className="sub">Service status</p>

      <div className={`banner ${banner.cls}`}>{banner.text}</div>

      {data.active_incidents.map((inc, i) => (
        <div className="incident active" key={i}>
          <div className="grow">
            <strong>{inc.monitor}</strong> is down
            <div className="meta">
              Since {fmtDate(inc.started_at)} · ongoing for {fmtDuration(inc.minutes)}
            </div>
          </div>
          <span className="pill down">Investigating</span>
        </div>
      ))}

      {data.monitors.map((m) => (
        <div className="item" key={m.name}>
          <div className="head">
            <div className="grow">
              <div className="name">{m.name}</div>
              <div className="meta">
                {m.uptime_90d.toFixed(2)}% uptime (90 days)
                {m.avg_latency_ms !== null && ` · ${m.avg_latency_ms} ms avg`}
              </div>
            </div>
            <span className={`pill ${m.status}`}>
              {m.status === 'up' ? 'Operational' : m.status === 'down' ? 'Down' : 'Pending'}
            </span>
          </div>
          {m.daily_uptime.length > 0 && (
            <>
              <UptimeBar daily={m.daily_uptime} />
              <div className="axis">
                <span>90 days ago</span>
                <span>Today (UTC)</span>
              </div>
            </>
          )}
        </div>
      ))}

      {!data.monitors.length && <p className="center">No services are being monitored yet.</p>}

      {data.recent_incidents.length > 0 && (
        <section>
          <h2>Past incidents</h2>
          {data.recent_incidents.map((inc, i) => (
            <div className="incident" key={i}>
              <div className="grow">
                <strong>{inc.monitor}</strong>
                <div className="meta">
                  {fmtDate(inc.started_at)} · resolved after {fmtDuration(inc.minutes)}
                </div>
              </div>
              <span className="pill up">Resolved</span>
            </div>
          ))}
        </section>
      )}

      <footer>
        Updated {new Date(data.updated_at).toLocaleString()} · Powered by Pulse
      </footer>
    </div>
  )
}

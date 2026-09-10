import { useEffect, useState } from 'react'

type Monitor = {
  name: string
  status: 'up' | 'down' | 'unknown'
  uptime_90d: number
  avg_latency_ms: number | null
}
type Status = { tenant: string; monitors: Monitor[]; updated_at: string }

// URL shape: /<slug> — e.g. status.pulse.io/northgate-digital
function slugFromPath(): string {
  return window.location.pathname.replace(/^\/+|\/+$/g, '')
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
    // Refresh every 60s. The endpoint sets Cache-Control 30s, so this is cheap.
    const t = window.setInterval(load, 60000)
    return () => window.clearInterval(t)
  }, [slug])

  if (err) return <div className="wrap"><p className="center">{err}</p></div>
  if (!data) return <div className="wrap"><p className="center">Loading…</p></div>

  const down = data.monitors.filter((m) => m.status === 'down')
  const banner = down.length
    ? { cls: 'bad', text: `${down.length} service${down.length > 1 ? 's' : ''} down` }
    : data.monitors.some((m) => m.status === 'up')
      ? { cls: 'ok', text: 'All systems operational' }
      : { cls: 'idle', text: 'Awaiting first checks' }

  return (
    <div className="wrap">
      <h1>{data.tenant}</h1>
      <p className="sub">Service status</p>

      <div className={`banner ${banner.cls}`}>{banner.text}</div>

      {data.monitors.map((m) => (
        <div className="item" key={m.name}>
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
      ))}

      {!data.monitors.length && <p className="center">No services are being monitored yet.</p>}

      <footer>
        Updated {new Date(data.updated_at).toLocaleString()} · Powered by Pulse
      </footer>
    </div>
  )
}

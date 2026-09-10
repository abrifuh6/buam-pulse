import { useCallback, useEffect, useState } from 'react'
import { api, token, type Monitor, type CheckResult } from './api'

function Sparkline({ id }: { id: string }) {
  const [results, setResults] = useState<CheckResult[]>([])

  useEffect(() => {
    let alive = true
    api
      .results(id)
      .then((r) => alive && setResults(r.slice(0, 30).reverse()))
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [id])

  if (!results.length) return <span className="metric">no data</span>

  // Distinguish "nothing recorded yet" from "every check is erroring".
  const lastErr = results[results.length - 1]?.error
  if (lastErr)
    return (
      <span className="metric" style={{ color: 'var(--down)' }} title={lastErr}>
        check failing
      </span>
    )

  const max = Math.max(...results.map((r) => r.latency_ms ?? 0), 1)
  return (
    <div className="spark" title="last 30 checks">
      {results.map((r, i) => (
        <i
          key={i}
          className={r.ok ? '' : 'bad'}
          style={{ height: `${Math.max(3, ((r.latency_ms ?? 0) / max) * 22)}px` }}
        />
      ))}
    </div>
  )
}

function AddMonitor({ onAdded }: { onAdded: () => void }) {
  const [name, setName] = useState('')
  const [type, setType] = useState('http')
  const [target, setTarget] = useState('')
  const [interval, setInterval] = useState(60)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  async function add() {
    setErr('')
    setBusy(true)
    try {
      await api.createMonitor({ name, type, target, interval_seconds: interval })
      setName('')
      setTarget('')
      onAdded()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="card">
      <div className="form-grid">
        <input placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
        <select value={type} onChange={(e) => setType(e.target.value)}>
          <option value="http">HTTP</option>
          <option value="tcp">TCP</option>
        </select>
        <input
          placeholder={type === 'http' ? 'https://example.com' : 'host:port'}
          value={target}
          onChange={(e) => setTarget(e.target.value)}
        />
        <select value={interval} onChange={(e) => setInterval(Number(e.target.value))}>
          <option value={30}>30s</option>
          <option value={60}>60s</option>
          <option value={300}>5m</option>
        </select>
        <button onClick={add} disabled={busy || !name || !target}>
          Add
        </button>
      </div>
      {err && <p className="err">{err}</p>}
    </div>
  )
}

export default function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [err, setErr] = useState('')

  const load = useCallback(async () => {
    try {
      setMonitors(await api.listMonitors())
    } catch (e) {
      if (e instanceof Error && e.message === 'session expired') return onLogout()
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }, [onLogout])

  // Poll every 15s so the page reflects check results without a reload.
  useEffect(() => {
    load()
    const t = window.setInterval(load, 15000)
    return () => window.clearInterval(t)
  }, [load])

  async function remove(id: string, name: string) {
    if (!confirm(`Delete monitor "${name}"?`)) return
    await api.deleteMonitor(id)
    load()
  }

  return (
    <div className="wrap">
      <header className="top">
        <h1 className="brand">
          Pulse <small>by Buam Technologies</small>
        </h1>
        <button
          className="ghost"
          onClick={() => {
            token.clear()
            onLogout()
          }}
        >
          Sign out
        </button>
      </header>

      <AddMonitor onAdded={load} />

      {err && <p className="err">{err}</p>}
      {!monitors.length && !err && (
        <p className="muted">No monitors yet. Add your first one above.</p>
      )}

      {monitors.map((m) => (
        <div className="card" key={m.id}>
          <div className="row">
            <span className={`dot ${m.status}`} title={m.status} />
            <div className="grow">
              <div className="name">{m.name}</div>
              <div className="target">{m.target}</div>
            </div>
            <Sparkline id={m.id} />
            <span className="metric">{m.interval_seconds}s</span>
            <button className="ghost" onClick={() => remove(m.id, m.name)}>
              Delete
            </button>
          </div>
        </div>
      ))}
    </div>
  )
}

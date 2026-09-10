import { useCallback, useEffect, useState } from 'react'
import { api, token, currentRole, type Monitor, type CheckResult } from './api'
import Channels from './Channels'
import Team from './Team'
import Billing from './Billing'

function Sparkline({ id, refreshKey }: { id: string; refreshKey: number }) {
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
  }, [id, refreshKey])

  if (!results.length) return <span className="metric">no data</span>

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

function MonitorRow({
  m,
  refreshKey,
  canWrite,
  onChanged,
}: {
  m: Monitor
  refreshKey: number
  canWrite: boolean
  onChanged: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(m.name)
  const [target, setTarget] = useState(m.target)
  const [interval, setInterval] = useState(m.interval_seconds)
  const [err, setErr] = useState('')

  async function save() {
    setErr('')
    try {
      await api.updateMonitor(m.id, { name, target, interval_seconds: interval })
      setEditing(false)
      onChanged()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }

  async function togglePause() {
    await api.updateMonitor(m.id, { enabled: !m.enabled })
    onChanged()
  }

  async function remove() {
    if (!confirm(`Delete monitor "${m.name}"?`)) return
    await api.deleteMonitor(m.id)
    onChanged()
  }

  if (editing)
    return (
      <div className="card">
        <div className="form-grid">
          <input value={name} onChange={(e) => setName(e.target.value)} />
          <span className="metric">{m.type.toUpperCase()}</span>
          <input value={target} onChange={(e) => setTarget(e.target.value)} />
          <select value={interval} onChange={(e) => setInterval(Number(e.target.value))}>
            <option value={30}>30s</option>
            <option value={60}>60s</option>
            <option value={300}>5m</option>
          </select>
          <button onClick={save}>Save</button>
        </div>
        <div style={{ marginTop: 10 }}>
          <button className="ghost" onClick={() => setEditing(false)}>
            Cancel
          </button>
        </div>
        {err && <p className="err">{err}</p>}
      </div>
    )

  return (
    <div className="card" style={{ opacity: m.enabled ? 1 : 0.55 }}>
      <div className="row">
        <span className={`dot ${m.enabled ? m.status : 'unknown'}`} title={m.status} />
        <div className="grow">
          <div className="name">
            {m.name}
            {!m.enabled && <span className="metric"> · paused</span>}
          </div>
          <div className="target">{m.target}</div>
        </div>
        {m.enabled && <Sparkline id={m.id} refreshKey={refreshKey} />}
        <span className="metric">{m.interval_seconds}s</span>
        {canWrite && (
          <>
            <button className="ghost" onClick={() => setEditing(true)}>
              Edit
            </button>
            <button className="ghost" onClick={togglePause}>
              {m.enabled ? 'Pause' : 'Resume'}
            </button>
            <button className="ghost" onClick={remove}>
              Delete
            </button>
          </>
        )}
      </div>
    </div>
  )
}

export default function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [tab, setTab] = useState<'monitors' | 'channels' | 'team' | 'billing'>('monitors')
  const role = currentRole()
  const canWrite = role === 'owner' || role === 'admin'
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [refreshKey, setRefreshKey] = useState(0)
  const [err, setErr] = useState('')

  const load = useCallback(async () => {
    try {
      setMonitors(await api.listMonitors())
      setRefreshKey((k) => k + 1)
    } catch (e) {
      if (e instanceof Error && e.message === 'session expired') return onLogout()
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }, [onLogout])

  useEffect(() => {
    load()
    const t = window.setInterval(load, 15000)
    return () => window.clearInterval(t)
  }, [load])

  return (
    <div className="wrap">
      <header className="top">
        <h1 className="brand">
          Pulse <small>by Buam Technologies</small>
        </h1>
        <div className="row">
          <button
            className="ghost"
            onClick={() => setTab('monitors')}
            style={{ color: tab === 'monitors' ? 'var(--text)' : undefined }}
          >
            Monitors
          </button>
          <button
            className="ghost"
            onClick={() => setTab('channels')}
            style={{ color: tab === 'channels' ? 'var(--text)' : undefined }}
          >
            Alerts
          </button>
          <button
            className="ghost"
            onClick={() => setTab('team')}
            style={{ color: tab === 'team' ? 'var(--text)' : undefined }}
          >
            Team
          </button>
          <button
            className="ghost"
            onClick={() => setTab('billing')}
            style={{ color: tab === 'billing' ? 'var(--text)' : undefined }}
          >
            Plan
          </button>
          <button
            className="ghost"
            onClick={() => {
              token.clear()
              onLogout()
            }}
          >
            Sign out
          </button>
        </div>
      </header>

      {tab === 'billing' ? (
        <Billing />
      ) : tab === 'team' ? (
        <Team />
      ) : tab === 'monitors' ? (
        <>
          {canWrite && <AddMonitor onAdded={load} />}
          {err && <p className="err">{err}</p>}
          {!monitors.length && !err && (
            <p className="muted">No monitors yet. Add your first one above.</p>
          )}
          {monitors.map((m) => (
            <MonitorRow
              key={m.id}
              m={m}
              refreshKey={refreshKey}
              canWrite={canWrite}
              onChanged={load}
            />
          ))}
        </>
      ) : (
        <Channels />
      )}
    </div>
  )
}

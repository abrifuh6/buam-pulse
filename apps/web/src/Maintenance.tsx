import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type MaintenanceWindow, type Monitor } from './api'

// datetime-local gives a value with no timezone; the browser interprets it as
// local time, and toISOString converts to UTC for the API. Getting this wrong
// is how a maintenance window ends up suppressing alerts eight hours late.
function toLocalInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(
    d.getHours(),
  )}:${pad(d.getMinutes())}`
}

function fmtRange(startISO: string, endISO: string): string {
  const s = new Date(startISO)
  const e = new Date(endISO)
  const opts: Intl.DateTimeFormatOptions = {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }
  return `${s.toLocaleString(undefined, opts)} → ${e.toLocaleString(undefined, opts)}`
}

export default function Maintenance({ monitors }: { monitors: Monitor[] }) {
  const canWrite = currentRole() === 'owner' || currentRole() === 'admin'

  const [windows, setWindows] = useState<MaintenanceWindow[]>([])
  const [title, setTitle] = useState('')
  const [starts, setStarts] = useState(() => toLocalInput(new Date(Date.now() + 3600_000)))
  const [ends, setEnds] = useState(() => toLocalInput(new Date(Date.now() + 7200_000)))
  const [selected, setSelected] = useState<string[]>([])
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      setWindows(await api.listMaintenance())
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function create() {
    setErr('')
    setBusy(true)
    try {
      await api.createMaintenance({
        title,
        starts_at: new Date(starts).toISOString(),
        ends_at: new Date(ends).toISOString(),
        monitor_ids: selected,
      })
      setTitle('')
      setSelected([])
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  function toggle(id: string) {
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]))
  }

  return (
    <>
      {canWrite && (
        <div className="card">
          <div className="stack">
            <input
              placeholder="What's happening? e.g. Database upgrade"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
            <div className="row">
              <label className="metric" style={{ width: 50 }}>
                From
              </label>
              <input
                type="datetime-local"
                value={starts}
                onChange={(e) => setStarts(e.target.value)}
              />
              <label className="metric" style={{ width: 30 }}>
                To
              </label>
              <input
                type="datetime-local"
                value={ends}
                onChange={(e) => setEnds(e.target.value)}
              />
            </div>

            <div>
              <div className="metric" style={{ marginBottom: 8 }}>
                {selected.length === 0
                  ? 'Applies to all monitors'
                  : `Applies to ${selected.length} selected`}
              </div>
              <div className="chips">
                {monitors.map((m) => (
                  <button
                    key={m.id}
                    className={`chip ${selected.includes(m.id) ? 'on' : ''}`}
                    onClick={() => toggle(m.id)}
                  >
                    {m.name}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <button onClick={create} disabled={busy || !title}>
                Schedule maintenance
              </button>
            </div>
          </div>
          {err && <p className="err">{err}</p>}
        </div>
      )}

      {!windows.length && (
        <p className="muted">
          No maintenance scheduled. During a window, alerts are suppressed and the downtime is
          marked as planned rather than counted against your uptime.
        </p>
      )}

      {windows.map((w) => (
        <div
          className="card"
          key={w.id}
          style={w.active ? { borderColor: 'var(--accent)' } : undefined}
        >
          <div className="row">
            <span className={`dot ${w.active ? 'up' : 'unknown'}`} />
            <div className="grow">
              <div className="name">
                {w.title}
                {w.active && <span className="metric"> · in progress</span>}
              </div>
              <div className="target">
                {fmtRange(w.starts_at, w.ends_at)} ·{' '}
                {w.monitor_ids.length === 0
                  ? 'all monitors'
                  : `${w.monitor_ids.length} monitor${w.monitor_ids.length > 1 ? 's' : ''}`}
              </div>
            </div>
            {canWrite && (
              <button
                className="ghost"
                onClick={async () => {
                  if (!confirm(`Delete "${w.title}"?`)) return
                  await api.deleteMaintenance(w.id)
                  load()
                }}
              >
                Delete
              </button>
            )}
          </div>
        </div>
      ))}
    </>
  )
}

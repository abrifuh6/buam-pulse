import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type MaintenanceWindow, type Monitor } from './api'

// datetime-local has no timezone; the browser reads it as local time and
// toISOString converts to UTC for the API. Getting this wrong is how a window
// suppresses alerts eight hours late.
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
  return `${s.toLocaleString(undefined, opts)} to ${e.toLocaleString(undefined, opts)}`
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
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    try {
      setWindows(await api.listMaintenance())
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not load maintenance windows.')
    } finally {
      setLoading(false)
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
      setErr(e instanceof Error ? e.message : 'Could not schedule this window.')
    } finally {
      setBusy(false)
    }
  }

  if (loading) return <div className="skeleton" style={{ height: 220 }} />

  return (
    <>
      {canWrite && (
        <div className="panel">
          <div className="panel-head">
            <h2 className="panel-title">Schedule a window</h2>
            <p className="panel-note">
              Alerts stay quiet and the downtime is recorded as planned, so it does not count
              against your published uptime.
            </p>
          </div>

          <div className="stack">
            <div className="field">
              <label htmlFor="mw-title">What is happening</label>
              <input
                id="mw-title"
                placeholder="Database upgrade"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
            </div>

            <div className="row">
              <div className="field grow">
                <label htmlFor="mw-from">Starts</label>
                <input
                  id="mw-from"
                  type="datetime-local"
                  value={starts}
                  onChange={(e) => setStarts(e.target.value)}
                />
              </div>
              <div className="field grow">
                <label htmlFor="mw-to">Ends</label>
                <input
                  id="mw-to"
                  type="datetime-local"
                  value={ends}
                  onChange={(e) => setEnds(e.target.value)}
                />
              </div>
            </div>

            <div className="field">
              <label>Applies to</label>
              <div className="chips">
                {monitors.map((m) => (
                  <button
                    key={m.id}
                    type="button"
                    className={`chip ${selected.includes(m.id) ? 'on' : ''}`}
                    onClick={() =>
                      setSelected((s) =>
                        s.includes(m.id) ? s.filter((x) => x !== m.id) : [...s, m.id],
                      )
                    }
                  >
                    {m.name}
                  </button>
                ))}
              </div>
              <span className="hint">
                {selected.length === 0
                  ? 'Nothing selected, so every monitor is covered.'
                  : `${selected.length} selected.`}
              </span>
            </div>

            <div>
              <button onClick={create} disabled={busy || !title}>
                {busy ? 'Scheduling…' : 'Schedule'}
              </button>
            </div>
          </div>
          {err && <p className="err">{err}</p>}
        </div>
      )}

      {!windows.length ? (
        <div className="empty">
          <h3>No planned work</h3>
          <p>
            Schedule a window before a deploy or a migration and Pulse will keep quiet while it
            runs, instead of paging you about downtime you caused on purpose.
          </p>
        </div>
      ) : (
        <div className="table-wrap">
          <table className="data">
            <thead>
              <tr>
                <th style={{ width: 28 }} />
                <th>Window</th>
                <th>Covers</th>
                <th className="right" style={{ width: 100 }} />
              </tr>
            </thead>
            <tbody>
              {windows.map((w) => (
                <tr key={w.id}>
                  <td>
                    <span className={`state ${w.active ? 'warn' : 'unknown'}`} />
                  </td>
                  <td>
                    <div className="cell-name">
                      {w.title}
                      {w.active && <span className="badge warn" style={{ marginLeft: 8 }}>in progress</span>}
                    </div>
                    <div className="cell-target">{fmtRange(w.starts_at, w.ends_at)}</div>
                  </td>
                  <td className="metric">
                    {w.monitor_ids.length === 0
                      ? 'all monitors'
                      : `${w.monitor_ids.length} monitor${w.monitor_ids.length > 1 ? 's' : ''}`}
                  </td>
                  <td className="right">
                    {canWrite && (
                      <button
                        className="quiet"
                        style={{ color: 'var(--down)' }}
                        onClick={async () => {
                          if (!confirm(`Delete "${w.title}"?`)) return
                          await api.deleteMaintenance(w.id)
                          load()
                        }}
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

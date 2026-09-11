import { useEffect, useState } from 'react'
import { api, type Channel, type CheckResult, type Monitor } from './api'
import Sparkline from './Sparkline'
import StatusStrip from './StatusStrip'
import MonitorForm from './MonitorForm'
import MonitorDetail from './MonitorDetail'
import { withTransition } from './transition'

function relative(iso: string | null): string {
  if (!iso) return '—'
  const secs = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
  if (secs < 60) return `${secs}s ago`
  if (secs < 3600) return `${Math.round(secs / 60)}m ago`
  if (secs < 86400) return `${Math.round(secs / 3600)}h ago`
  return `${Math.round(secs / 86400)}d ago`
}

function interval(seconds: number): string {
  return seconds >= 60 ? `${seconds / 60}m` : `${seconds}s`
}

type RowData = { results: CheckResult[]; last: string | null }

export default function Monitors({
  monitors,
  channels,
  canWrite,
  loading,
  openMonitor,
  onOpened,
  onChanged,
}: {
  monitors: Monitor[]
  channels: Channel[]
  canWrite: boolean
  loading: boolean
  openMonitor?: string | null
  onOpened?: () => void
  onChanged: () => void
}) {
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Monitor | null>(null)
  const [viewing, setViewing] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [data, setData] = useState<Record<string, RowData>>({})

  // An incident click hands us a monitor to open.
  useEffect(() => {
    if (openMonitor) {
      setViewing(openMonitor)
      onOpened?.()
    }
  }, [openMonitor, onOpened])

  // Keyboard: / focuses search, n adds a monitor. Both are ignored while
  // typing, so they never swallow a character mid-word.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement
      if (el.tagName === 'INPUT' || el.tagName === 'SELECT' || el.tagName === 'TEXTAREA') return
      if (e.key === '/') {
        e.preventDefault()
        document.getElementById('monitor-search')?.focus()
      }
      if (e.key === 'n' && canWrite) setAdding(true)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [canWrite])

  // One request per monitor for its recent history. Batched into a single
  // effect keyed on the monitor list so a poll does not stack requests.
  useEffect(() => {
    let alive = true
    Promise.all(
      monitors.map(async (m) => {
        try {
          const r = await api.results(m.id)
          return [m.id, { results: r.slice(0, 24).reverse(), last: r[0]?.checked_at ?? null }] as const
        } catch {
          return [m.id, { results: [], last: null }] as const
        }
      }),
    ).then((entries) => {
      if (alive) setData(Object.fromEntries(entries))
    })
    return () => {
      alive = false
    }
  }, [monitors])

  const current = monitors.find((m) => m.id === viewing)
  if (current) {
    return (
      <MonitorDetail
        monitor={current}
        channels={channels}
        canWrite={canWrite}
        onClose={() => withTransition(() => setViewing(null))}
        onChanged={onChanged}
      />
    )
  }

  if (loading) {
    return (
      <>
        <div className="skeleton" style={{ height: 84, marginBottom: 'var(--s-5)' }} />
        <div className="skeleton" style={{ height: 240 }} />
      </>
    )
  }

  if (!monitors.length) {
    return (
      <>
        <StatusStrip monitors={monitors} />
        <div className="empty">
          <h3>Start watching something</h3>
          <p>
            Add the URL of a site or API. Pulse checks it on a schedule and tells you the
            moment it stops responding.
          </p>
          {canWrite && <button onClick={() => setAdding(true)}>Add your first monitor</button>}
        </div>
        {adding && (
          <MonitorForm
            channels={channels}
            onClose={() => setAdding(false)}
            onSaved={() => {
              setAdding(false)
              onChanged()
            }}
          />
        )}
      </>
    )
  }

  const q = query.trim().toLowerCase()
  const shown = q
    ? monitors.filter(
        (m) => m.name.toLowerCase().includes(q) || m.target.toLowerCase().includes(q),
      )
    : monitors

  return (
    <>
      <StatusStrip monitors={monitors} />

      <div className="spread" style={{ marginBottom: 'var(--s-3)' }}>
        <input
          id="monitor-search"
          className="search"
          placeholder="Search monitors…  /"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {canWrite && <button onClick={() => setAdding(true)}>Add monitor</button>}
      </div>

      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr>
              <th style={{ width: 28 }} />
              <th>Name</th>
              <th className="right">Every</th>
              <th className="right">Latency</th>
              <th style={{ width: 110 }}>Last 24 checks</th>
              <th className="right">Checked</th>
              <th style={{ width: 60 }} />
            </tr>
          </thead>
          <tbody>
            {shown.map((m) => {
              const d = data[m.id]
              const lastResult = d?.results[d.results.length - 1]
              const latency = lastResult?.latency_ms
              return (
                <tr
                  key={m.id}
                  className={`clickable ${m.enabled ? '' : 'is-paused'}`}
                  onClick={() => withTransition(() => setViewing(m.id))}
                >
                  <td>
                    <span
                      className={`state ${m.enabled ? m.status : 'unknown'}`}
                      title={m.enabled ? m.status : 'paused'}
                    />
                  </td>
                  <td>
                    <div className="cell-name">{m.name}</div>
                    <div className="cell-target">{m.target}</div>
                  </td>
                  <td className="right num">{interval(m.interval_seconds)}</td>
                  <td className="right num">
                    {latency != null ? `${latency}ms` : '—'}
                  </td>
                  <td>{d ? <Sparkline results={d.results} /> : null}</td>
                  <td className="right num">{relative(d?.last ?? null)}</td>
                  <td className="right">
                    {canWrite && (
                      <button
                        className="quiet"
                        onClick={(e) => {
                          e.stopPropagation()
                          setEditing(m)
                        }}
                      >
                        Edit
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {(adding || editing) && (
        <MonitorForm
          monitor={editing ?? undefined}
          channels={channels}
          onClose={() => {
            setAdding(false)
            setEditing(null)
          }}
          onSaved={() => {
            setAdding(false)
            setEditing(null)
            onChanged()
          }}
        />
      )}
    </>
  )
}

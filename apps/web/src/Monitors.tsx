import { useEffect, useState } from 'react'
import { api, type Channel, type CheckResult, type Monitor } from './api'
import Sparkline from './Sparkline'
import StatusStrip from './StatusStrip'
import MonitorForm from './MonitorForm'
import MonitorDetail from './MonitorDetail'

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
  onChanged,
}: {
  monitors: Monitor[]
  channels: Channel[]
  canWrite: boolean
  loading: boolean
  onChanged: () => void
}) {
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Monitor | null>(null)
  const [viewing, setViewing] = useState<string | null>(null)
  const [data, setData] = useState<Record<string, RowData>>({})

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
        onClose={() => setViewing(null)}
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

  return (
    <>
      <StatusStrip monitors={monitors} />

      <div className="spread" style={{ marginBottom: 'var(--s-3)' }}>
        <span className="metric">
          {monitors.length} {monitors.length === 1 ? 'monitor' : 'monitors'}
        </span>
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
            {monitors.map((m) => {
              const d = data[m.id]
              const lastResult = d?.results[d.results.length - 1]
              const latency = lastResult?.latency_ms
              return (
                <tr
                  key={m.id}
                  className={`clickable ${m.enabled ? '' : 'is-paused'}`}
                  onClick={() => setViewing(m.id)}
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

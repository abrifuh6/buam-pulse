import { useEffect, useState } from 'react'
import { api, type IncidentRow } from './api'

function fmtDuration(mins: number): string {
  if (mins < 60) return `${mins}m`
  const h = Math.floor(mins / 60)
  return h < 24 ? `${h}h ${mins % 60}m` : `${Math.floor(h / 24)}d ${h % 24}h`
}

export default function Incidents({ onOpenMonitor }: { onOpenMonitor: (id: string) => void }) {
  const [rows, setRows] = useState<IncidentRow[] | null>(null)
  const [showPlanned, setShowPlanned] = useState(false)

  useEffect(() => {
    api.listIncidents().then(setRows).catch(() => setRows([]))
  }, [])

  if (!rows) return <div className="skeleton" style={{ height: 260 }} />

  const visible = showPlanned ? rows : rows.filter((r) => !r.planned)
  const ongoing = visible.filter((r) => !r.resolved_at)
  const plannedCount = rows.filter((r) => r.planned).length

  // Total downtime over the visible set, excluding planned work — the number
  // you would quote in a review.
  const totalMinutes = visible
    .filter((r) => !r.planned && r.resolved_at)
    .reduce((sum, r) => sum + r.minutes, 0)

  if (!rows.length) {
    return (
      <div className="empty">
        <h3>Nothing has broken</h3>
        <p>
          When a monitor fails twice in a row, the outage is recorded here with how long it
          lasted and whether anyone was told.
        </p>
      </div>
    )
  }

  return (
    <>
      <div className="detail-stats" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
        <div className="stat">
          <b style={{ color: ongoing.length ? 'var(--down)' : undefined }}>{ongoing.length}</b>
          <span>happening now</span>
        </div>
        <div className="stat">
          <b>{visible.filter((r) => !r.planned).length}</b>
          <span>recorded outages</span>
        </div>
        <div className="stat">
          <b>{fmtDuration(totalMinutes)}</b>
          <span>total downtime</span>
        </div>
      </div>

      {plannedCount > 0 && (
        <div className="spread" style={{ marginBottom: 'var(--s-3)' }}>
          <span className="metric">
            {plannedCount} planned {plannedCount === 1 ? 'window' : 'windows'} hidden
          </span>
          <button className="quiet" onClick={() => setShowPlanned(!showPlanned)}>
            {showPlanned ? 'Hide planned' : 'Show planned'}
          </button>
        </div>
      )}

      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr>
              <th style={{ width: 28 }} />
              <th>Monitor</th>
              <th>Started</th>
              <th className="right">Lasted</th>
              <th>Cause</th>
              <th className="right" style={{ width: 110 }}>
                Alert
              </th>
            </tr>
          </thead>
          <tbody>
            {visible.map((r) => (
              <tr key={r.id} className="clickable" onClick={() => onOpenMonitor(r.monitor_id)}>
                <td>
                  <span className={`state ${r.resolved_at ? 'unknown' : 'down'}`} />
                </td>
                <td className="cell-name">{r.monitor}</td>
                <td className="num">{new Date(r.started_at).toLocaleString()}</td>
                <td className="right num">
                  {fmtDuration(r.minutes)}
                  {!r.resolved_at && <span className="metric"> so far</span>}
                </td>
                <td className="cell-target">{r.cause ?? '—'}</td>
                <td className="right">
                  {r.planned ? (
                    <span className="badge idle">planned</span>
                  ) : r.notified ? (
                    <span className="badge up">sent</span>
                  ) : (
                    <span className="badge warn">nobody told</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}

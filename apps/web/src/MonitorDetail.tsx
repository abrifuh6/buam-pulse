import { useEffect, useState } from 'react'
import { api, type Channel, type Monitor, type MonitorDetail as Detail } from './api'
import LatencyChart from './LatencyChart'
import MonitorForm from './MonitorForm'

type Window = '24h' | '7d' | '30d'

function fmtDuration(mins: number): string {
  if (mins < 60) return `${mins}m`
  const h = Math.floor(mins / 60)
  return h < 24 ? `${h}h ${mins % 60}m` : `${Math.floor(h / 24)}d ${h % 24}h`
}

function certState(iso: string | null): { label: string; cls: string } | null {
  if (!iso) return null
  const days = Math.round((new Date(iso).getTime() - Date.now()) / 86400000)
  if (days < 0) return { label: 'expired', cls: 'down' }
  if (days <= 14) return { label: `${days}d left`, cls: 'warn' }
  return { label: `${days}d left`, cls: 'up' }
}

export default function MonitorDetail({
  monitor,
  channels,
  canWrite,
  onClose,
  onChanged,
}: {
  monitor: Monitor
  channels: Channel[]
  canWrite: boolean
  onClose: () => void
  onChanged: () => void
}) {
  const [window, setWindow] = useState<Window>('24h')
  const [detail, setDetail] = useState<Detail | null>(null)
  const [editing, setEditing] = useState(false)

  useEffect(() => {
    let alive = true
    setDetail(null)
    api
      .monitorDetail(monitor.id, window)
      .then((d) => alive && setDetail(d))
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [monitor.id, window])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && !editing && onClose()
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose, editing])

  const cert = certState(monitor.ssl_expires_at)
  const uptime =
    detail && (window === '24h' ? detail.uptime_24h : window === '7d' ? detail.uptime_7d : detail.uptime_30d)

  return (
    <div className="detail">
      <div className="detail-head">
        <button className="quiet" onClick={onClose}>
          ← Monitors
        </button>
      </div>

      <div className="spread" style={{ marginBottom: 'var(--s-5)' }}>
        <div className="row">
          <span className={`state ${monitor.enabled ? monitor.status : 'unknown'}`} />
          <div>
            <h1 className="page-title">{monitor.name}</h1>
            <p className="page-sub">{monitor.target}</p>
          </div>
        </div>
        <div className="row">
          <div className="window-picker">
            {(['24h', '7d', '30d'] as Window[]).map((wd) => (
              <button key={wd} className={window === wd ? 'on' : ''} onClick={() => setWindow(wd)}>
                {wd}
              </button>
            ))}
          </div>
          {canWrite && <button onClick={() => setEditing(true)}>Settings</button>}
        </div>
      </div>

      <div className="detail-stats">
        <div className="stat">
          <b style={{ color: uptime != null && uptime < 99 ? 'var(--down)' : undefined }}>
            {uptime != null ? `${uptime.toFixed(2)}%` : '—'}
          </b>
          <span>uptime, last {window}</span>
        </div>
        <div className="stat">
          <b>{detail?.avg_latency_ms != null ? `${detail.avg_latency_ms}ms` : '—'}</b>
          <span>average response</span>
        </div>
        <div className="stat">
          <b>{detail?.p95_latency_ms != null ? `${detail.p95_latency_ms}ms` : '—'}</b>
          <span>95th percentile</span>
        </div>
        <div className="stat">
          {cert ? (
            <>
              <b style={{ color: cert.cls === 'up' ? undefined : `var(--${cert.cls})` }}>
                {cert.label}
              </b>
              <span>TLS certificate{monitor.ssl_issuer ? `, ${monitor.ssl_issuer}` : ''}</span>
            </>
          ) : (
            <>
              <b>—</b>
              <span>no certificate seen</span>
            </>
          )}
        </div>
      </div>

      {detail ? (
        <LatencyChart series={detail.series} />
      ) : (
        <div className="skeleton" style={{ height: 240 }} />
      )}

      <p className="section-label">Incidents</p>

      {detail && detail.incidents.length === 0 ? (
        <div className="empty">
          <h3>No incidents recorded</h3>
          <p>This monitor has not failed since it was created.</p>
        </div>
      ) : (
        <div className="table-wrap">
          <table className="data">
            <thead>
              <tr>
                <th style={{ width: 28 }} />
                <th>Started</th>
                <th>Lasted</th>
                <th>Cause</th>
                <th className="right">Alerted</th>
              </tr>
            </thead>
            <tbody>
              {detail?.incidents.map((inc, i) => (
                <tr key={i}>
                  <td>
                    <span
                      className={`state ${inc.resolved_at ? 'unknown' : 'down'}`}
                      title={inc.resolved_at ? 'resolved' : 'ongoing'}
                    />
                  </td>
                  <td className="num">{new Date(inc.started_at).toLocaleString()}</td>
                  <td className="num">
                    {fmtDuration(inc.minutes)}
                    {!inc.resolved_at && <span className="metric"> and counting</span>}
                  </td>
                  <td className="cell-target">{inc.cause ?? '—'}</td>
                  <td className="right">
                    {inc.planned ? (
                      <span className="badge idle">planned</span>
                    ) : inc.notified ? (
                      <span className="badge up">sent</span>
                    ) : (
                      <span className="badge warn">no channel</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {editing && (
        <MonitorForm
          monitor={monitor}
          channels={channels}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false)
            onChanged()
          }}
        />
      )}
    </div>
  )
}

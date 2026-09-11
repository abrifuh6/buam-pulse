import type { Monitor } from './api'

// The one bold moment in the interface. It answers "is anything wrong" before
// anything else is read, and changes character entirely when the answer is yes.
export default function StatusStrip({ monitors }: { monitors: Monitor[] }) {
  const active = monitors.filter((m) => m.enabled)
  const down = active.filter((m) => m.status === 'down')
  const pending = active.filter((m) => m.status === 'unknown')

  const headline =
    down.length > 0
      ? `${down.length} ${down.length === 1 ? 'monitor is' : 'monitors are'} down`
      : active.length === 0
        ? 'Nothing is being monitored'
        : 'All systems operational'

  const detail =
    down.length > 0
      ? down.map((m) => m.name).join(', ')
      : pending.length > 0
        ? `${pending.length} awaiting a first check`
        : `${active.length} ${active.length === 1 ? 'monitor' : 'monitors'} healthy`

  return (
    <div className={`strip ${down.length > 0 ? 'is-down' : ''}`}>
      <div className="grow">
        <div className="strip-headline">{headline}</div>
        <div className="strip-detail">{detail}</div>
      </div>
      <div className="strip-stats">
        <div className="strip-stat">
          <b>{active.length}</b>
          <span>monitored</span>
        </div>
        <div className="strip-stat">
          <b style={{ color: down.length ? 'var(--down)' : undefined }}>{down.length}</b>
          <span>down</span>
        </div>
        <div className="strip-stat">
          <b>{monitors.length - active.length}</b>
          <span>paused</span>
        </div>
      </div>
    </div>
  )
}

import { useEffect, useState } from 'react'
import { api, type Channel, type Monitor } from './api'

// One panel for creating and editing, rather than an inline row: a monitor has
// eleven settings, and squeezing them into a table row is what pushed keyword
// and routing out of sight in the first place.
export default function MonitorForm({
  monitor,
  channels,
  onClose,
  onSaved,
}: {
  monitor?: Monitor
  channels: Channel[]
  onClose: () => void
  onSaved: () => void
}) {
  const editing = !!monitor

  const [name, setName] = useState(monitor?.name ?? '')
  const [type, setType] = useState<'http' | 'tcp'>(monitor?.type ?? 'http')
  const [target, setTarget] = useState(monitor?.target ?? '')
  const [interval, setIntervalSecs] = useState(monitor?.interval_seconds ?? 60)
  const [keyword, setKeyword] = useState(monitor?.keyword ?? '')
  const [keywordPresent, setKeywordPresent] = useState(monitor?.keyword_present ?? true)
  const [delay, setDelay] = useState(monitor?.alert_delay_seconds ?? 0)
  const [routed, setRouted] = useState<string[] | null>(editing ? null : [])
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!editing || !monitor) return
    api
      .monitorChannels(monitor.id)
      .then((r) => setRouted(r.channel_ids))
      .catch(() => setRouted([]))
  }, [editing, monitor])

  // Escape closes, which every dialog should honour.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  async function save() {
    setErr('')
    setBusy(true)
    try {
      if (editing && monitor) {
        await api.updateMonitor(monitor.id, {
          name,
          target,
          interval_seconds: interval,
          alert_delay_seconds: delay,
          keyword,
          keyword_present: keywordPresent,
        })
        if (routed !== null) await api.setMonitorChannels(monitor.id, routed)
      } else {
        const created = await api.createMonitor({
          name,
          type,
          target,
          interval_seconds: interval,
          keyword,
          keyword_present: keywordPresent,
          alert_delay_seconds: delay,
        })
        if (routed && routed.length) await api.setMonitorChannels(created.id, routed)
      }
      onSaved()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not save this monitor.')
    } finally {
      setBusy(false)
    }
  }

  async function togglePause() {
    if (!monitor) return
    await api.updateMonitor(monitor.id, { enabled: !monitor.enabled })
    onSaved()
  }

  async function remove() {
    if (!monitor) return
    if (!confirm(`Delete "${monitor.name}"? Its history goes too.`)) return
    await api.deleteMonitor(monitor.id)
    onSaved()
  }

  return (
    <div className="sheet-backdrop" onClick={onClose}>
      <div className="sheet" onClick={(e) => e.stopPropagation()}>
        <div className="sheet-head">
          <h2 className="panel-title">{editing ? monitor?.name : 'New monitor'}</h2>
          <button className="quiet" onClick={onClose}>
            Close
          </button>
        </div>

        <div className="sheet-body stack">
          <div className="field">
            <label htmlFor="mf-name">Name</label>
            <input
              id="mf-name"
              placeholder="Checkout API"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          <div className="row">
            <div className="field" style={{ width: 110 }}>
              <label htmlFor="mf-type">Type</label>
              <select
                id="mf-type"
                value={type}
                onChange={(e) => setType(e.target.value as 'http' | 'tcp')}
                disabled={editing}
              >
                <option value="http">HTTP</option>
                <option value="tcp">TCP</option>
              </select>
            </div>
            <div className="field grow">
              <label htmlFor="mf-target">{type === 'http' ? 'URL' : 'Host and port'}</label>
              <input
                id="mf-target"
                placeholder={type === 'http' ? 'https://example.com' : 'db.example.com:5432'}
                value={target}
                onChange={(e) => setTarget(e.target.value)}
              />
            </div>
          </div>

          <div className="row">
            <div className="field grow">
              <label htmlFor="mf-interval">Check every</label>
              <select
                id="mf-interval"
                value={interval}
                onChange={(e) => setIntervalSecs(Number(e.target.value))}
              >
                <option value={30}>30 seconds</option>
                <option value={60}>1 minute</option>
                <option value={300}>5 minutes</option>
                <option value={900}>15 minutes</option>
              </select>
            </div>
            <div className="field grow">
              <label htmlFor="mf-delay">Alert after</label>
              <select
                id="mf-delay"
                value={delay}
                onChange={(e) => setDelay(Number(e.target.value))}
              >
                <option value={0}>the first confirmed failure</option>
                <option value={120}>2 minutes down</option>
                <option value={300}>5 minutes down</option>
                <option value={900}>15 minutes down</option>
              </select>
            </div>
          </div>

          {type === 'http' && (
            <div className="field">
              <label htmlFor="mf-keyword">Page content</label>
              <div className="row">
                <select
                  value={keywordPresent ? 'contains' : 'missing'}
                  onChange={(e) => setKeywordPresent(e.target.value === 'contains')}
                  style={{ width: 150 }}
                >
                  <option value="contains">must contain</option>
                  <option value="missing">must not contain</option>
                </select>
                <input
                  id="mf-keyword"
                  placeholder="optional — e.g. Add to cart"
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                />
              </div>
              <span className="hint">
                A page can answer 200 while showing an error. Checking for text catches that.
              </span>
            </div>
          )}

          {channels.length > 0 && (
            <div className="field">
              <label>Send alerts to</label>
              <div className="chips">
                {channels.map((ch) => (
                  <button
                    key={ch.id}
                    type="button"
                    className={`chip ${routed?.includes(ch.id) ? 'on' : ''}`}
                    disabled={routed === null}
                    onClick={() =>
                      setRouted((cur) =>
                        cur === null
                          ? cur
                          : cur.includes(ch.id)
                            ? cur.filter((x) => x !== ch.id)
                            : [...cur, ch.id],
                      )
                    }
                  >
                    {ch.name}
                  </button>
                ))}
              </div>
              <span className="hint">
                {routed === null
                  ? 'Loading…'
                  : routed.length === 0
                    ? 'Nothing selected, so alerts go to every channel.'
                    : `${routed.length} selected.`}
              </span>
            </div>
          )}

          {err && <p className="err">{err}</p>}
        </div>

        <div className="sheet-foot">
          {editing && (
            <>
              <button className="ghost" onClick={togglePause}>
                {monitor?.enabled ? 'Pause checks' : 'Resume checks'}
              </button>
              <button className="quiet" style={{ color: 'var(--down)' }} onClick={remove}>
                Delete
              </button>
            </>
          )}
          <span className="grow" />
          <button onClick={save} disabled={busy || !name || !target}>
            {busy ? 'Saving…' : editing ? 'Save changes' : 'Add monitor'}
          </button>
        </div>
      </div>
    </div>
  )
}

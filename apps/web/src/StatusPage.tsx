import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type Monitor, type StatusPageSettings } from './api'

export default function StatusPage({
  monitors,
  onChanged,
}: {
  monitors: Monitor[]
  onChanged: () => void
}) {
  const canWrite = currentRole() === 'owner' || currentRole() === 'admin'

  const [s, setS] = useState<StatusPageSettings | null>(null)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [supportURL, setSupportURL] = useState('')
  const [copied, setCopied] = useState(false)
  const [err, setErr] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const v = await api.statusPageSettings()
      setS(v)
      setTitle(v.title ?? '')
      setDescription(v.description ?? '')
      setSupportURL(v.support_url ?? '')
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not load these settings.')
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function save() {
    setErr('')
    setNote('')
    setBusy(true)
    try {
      await api.updateStatusPage({ title, description, support_url: supportURL })
      setNote('Saved.')
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not save.')
    } finally {
      setBusy(false)
    }
  }

  if (!s) return <div className="skeleton" style={{ height: 260 }} />

  const publicCount = monitors.filter((m) => m.public).length

  return (
    <>
      <div className="panel">
        <div className="spread" style={{ marginBottom: 'var(--s-4)' }}>
          <div>
            <h2 className="panel-title">Your public page</h2>
            <p className="panel-note">{s.public_url}</p>
          </div>
          <div className="row">
            <button
              className="ghost"
              onClick={async () => {
                await navigator.clipboard.writeText(s.public_url)
                setCopied(true)
                setTimeout(() => setCopied(false), 1600)
              }}
            >
              {copied ? 'Copied' : 'Copy link'}
            </button>
            <a className="chip" href={s.public_url} target="_blank" rel="noreferrer">
              Open
            </a>
          </div>
        </div>

        <div className="stack">
          <div className="field">
            <label htmlFor="sp-title">Heading</label>
            <input
              id="sp-title"
              placeholder="Defaults to your organization name"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              disabled={!canWrite}
            />
          </div>
          <div className="field">
            <label htmlFor="sp-desc">Subheading</label>
            <input
              id="sp-desc"
              placeholder="Status of our hosted services"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={!canWrite}
            />
          </div>
          <div className="field">
            <label htmlFor="sp-support">Support link</label>
            <input
              id="sp-support"
              placeholder="https://yoursite.com/support"
              value={supportURL}
              onChange={(e) => setSupportURL(e.target.value)}
              disabled={!canWrite}
            />
            <span className="hint">Shown in the footer so visitors can reach you.</span>
          </div>

          {canWrite && (
            <div className="row">
              <button onClick={save} disabled={busy}>
                {busy ? 'Saving…' : 'Save changes'}
              </button>
              <button
                className="ghost"
                disabled={!s.can_hide_branding}
                title={s.can_hide_branding ? '' : 'Available on paid plans'}
                onClick={async () => {
                  await api.updateStatusPage({ hide_branding: !s.hide_branding })
                  load()
                }}
              >
                {s.hide_branding ? 'Show Pulse branding' : 'Hide Pulse branding'}
              </button>
            </div>
          )}
        </div>

        {err && <p className="err">{err}</p>}
        {note && <p className="ok-note">{note}</p>}
      </div>

      <p className="section-label">
        {publicCount} of {monitors.length} monitors are shown publicly
      </p>

      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr>
              <th style={{ width: 28 }} />
              <th>Monitor</th>
              <th>On your page</th>
              <th className="right" style={{ width: 90 }} />
            </tr>
          </thead>
          <tbody>
            {monitors.map((m) => (
              <tr key={m.id} className={m.public ? '' : 'is-paused'}>
                <td>
                  <span className={`state ${m.public ? 'up' : 'unknown'}`} />
                </td>
                <td>
                  <div className="cell-name">{m.public_name || m.name}</div>
                  <div className="cell-target">{m.target}</div>
                </td>
                <td>
                  <span className={`badge ${m.public ? 'up' : 'idle'}`}>
                    {m.public ? 'visible' : 'private'}
                  </span>
                </td>
                <td className="right">
                  {canWrite && (
                    <button
                      className="quiet"
                      onClick={async () => {
                        await api.updateMonitor(m.id, { public: !m.public })
                        onChanged()
                      }}
                    >
                      {m.public ? 'Hide' : 'Show'}
                    </button>
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

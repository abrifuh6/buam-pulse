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
      setErr(e instanceof Error ? e.message : 'failed')
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
      setErr(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  async function toggleBranding() {
    if (!s) return
    setErr('')
    try {
      await api.updateStatusPage({ hide_branding: !s.hide_branding })
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }

  // Visibility is a property of the monitor, so it is saved immediately rather
  // than batched with the page settings — a toggle that needs a separate Save
  // is a toggle people forget to save.
  async function toggleVisible(m: Monitor) {
    await api.updateMonitor(m.id, { public: !m.public })
    onChanged()
  }

  if (!s) return <p className="muted">Loading…</p>

  return (
    <>
      <div className="card">
        <div className="row" style={{ marginBottom: 14 }}>
          <div className="grow">
            <div className="name">Your public status page</div>
            <div className="target">{s.public_url}</div>
          </div>
          <a className="chip" href={s.public_url} target="_blank" rel="noreferrer">
            Open
          </a>
        </div>

        <div className="stack">
          <input
            placeholder="Page title (defaults to your organization name)"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            disabled={!canWrite}
          />
          <input
            placeholder="Subtitle, e.g. Status of Northgate's hosted services"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={!canWrite}
          />
          <input
            placeholder="Support link, e.g. https://yoursite.com/support"
            value={supportURL}
            onChange={(e) => setSupportURL(e.target.value)}
            disabled={!canWrite}
          />
          {canWrite && (
            <div className="row">
              <button onClick={save} disabled={busy}>
                Save
              </button>
              <button
                className="ghost"
                onClick={toggleBranding}
                disabled={!s.can_hide_branding}
                title={
                  s.can_hide_branding
                    ? ''
                    : 'Removing Pulse branding requires a paid plan'
                }
              >
                {s.hide_branding ? 'Show "Powered by Pulse"' : 'Hide "Powered by Pulse"'}
              </button>
            </div>
          )}
        </div>

        {err && <p className="err">{err}</p>}
        {note && (
          <p className="muted" style={{ marginTop: 10, fontSize: 13 }}>
            {note}
          </p>
        )}
      </div>

      <p className="muted" style={{ marginTop: 24, fontSize: 13 }}>
        Which monitors appear publicly
      </p>

      {monitors.map((m) => (
        <div className="card" key={m.id} style={{ opacity: m.public ? 1 : 0.55 }}>
          <div className="row">
            <span className={`dot ${m.public ? 'up' : 'unknown'}`} />
            <div className="grow">
              <div className="name">{m.public_name || m.name}</div>
              <div className="target">
                {m.public ? 'visible on your status page' : 'hidden — monitored privately'}
              </div>
            </div>
            {canWrite && (
              <button className="ghost" onClick={() => toggleVisible(m)}>
                {m.public ? 'Hide' : 'Show'}
              </button>
            )}
          </div>
        </div>
      ))}
    </>
  )
}

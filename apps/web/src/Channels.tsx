import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type Channel } from './api'

export default function Channels() {
  const canWrite = currentRole() === 'owner' || currentRole() === 'admin'

  const [channels, setChannels] = useState<Channel[]>([])
  const [type, setType] = useState<'email' | 'slack'>('email')
  const [name, setName] = useState('')
  const [value, setValue] = useState('')
  const [err, setErr] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    try {
      setChannels(await api.listChannels())
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not load channels.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function add() {
    setErr('')
    setNote('')
    setBusy(true)
    try {
      const res = await api.createChannel({
        name,
        type,
        ...(type === 'email' ? { address: value } : { webhook_url: value }),
      })
      setName('')
      setValue('')
      setNote(
        res.verification_sent
          ? 'Check that inbox and confirm the address before alerts are sent to it.'
          : 'Channel added.',
      )
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not add this channel.')
    } finally {
      setBusy(false)
    }
  }

  async function test(id: string) {
    setErr('')
    setNote('')
    try {
      await api.testChannel(id)
      setNote('Test alert sent.')
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Delivery failed.')
    }
  }

  if (loading) return <div className="skeleton" style={{ height: 200 }} />

  return (
    <>
      {canWrite && (
        <div className="panel">
          <div className="panel-head">
            <h2 className="panel-title">Add a channel</h2>
            <p className="panel-note">
              Email addresses are confirmed before anything is sent to them.
            </p>
          </div>
          <div className="stack">
            <div className="row">
              <div className="field grow">
                <label htmlFor="ch-name">Name</label>
                <input
                  id="ch-name"
                  placeholder="Ops inbox"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>
              <div className="field" style={{ width: 130 }}>
                <label htmlFor="ch-type">Type</label>
                <select
                  id="ch-type"
                  value={type}
                  onChange={(e) => setType(e.target.value as 'email' | 'slack')}
                >
                  <option value="email">Email</option>
                  <option value="slack">Slack</option>
                </select>
              </div>
            </div>
            <div className="field">
              <label htmlFor="ch-value">
                {type === 'email' ? 'Address' : 'Incoming webhook URL'}
              </label>
              <input
                id="ch-value"
                placeholder={
                  type === 'email' ? 'ops@yourcompany.com' : 'https://hooks.slack.com/services/…'
                }
                value={value}
                onChange={(e) => setValue(e.target.value)}
              />
            </div>
            <div>
              <button onClick={add} disabled={busy || !name || !value}>
                {busy ? 'Adding…' : 'Add channel'}
              </button>
            </div>
          </div>
          {err && <p className="err">{err}</p>}
          {note && <p className="ok-note">{note}</p>}
        </div>
      )}

      {!channels.length ? (
        <div className="empty">
          <h3>Nowhere to send alerts</h3>
          <p>
            Pulse is watching your monitors, but with no channel it has no way to reach you when
            one fails. Add an email address or a Slack webhook.
          </p>
        </div>
      ) : (
        <div className="table-wrap">
          <table className="data">
            <thead>
              <tr>
                <th style={{ width: 28 }} />
                <th>Channel</th>
                <th>Type</th>
                <th className="right" style={{ width: 150 }} />
              </tr>
            </thead>
            <tbody>
              {channels.map((c) => (
                <tr key={c.id}>
                  <td>
                    <span
                      className={`state ${c.verified ? 'up' : 'warn'}`}
                      title={c.verified ? 'confirmed' : 'awaiting confirmation'}
                    />
                  </td>
                  <td>
                    <div className="cell-name">{c.name}</div>
                    <div className="cell-target">
                      {c.type === 'email' ? c.config.address : c.config.webhook_url}
                    </div>
                  </td>
                  <td>
                    <span className={`badge ${c.verified ? 'up' : 'warn'}`}>
                      {c.verified ? c.type : 'unconfirmed'}
                    </span>
                  </td>
                  <td className="right">
                    {canWrite && (
                      <>
                        <button className="quiet" onClick={() => test(c.id)} disabled={!c.verified}>
                          Test
                        </button>
                        <button
                          className="quiet"
                          style={{ color: 'var(--down)' }}
                          onClick={async () => {
                            if (!confirm(`Delete "${c.name}"?`)) return
                            await api.deleteChannel(c.id)
                            load()
                          }}
                        >
                          Delete
                        </button>
                      </>
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

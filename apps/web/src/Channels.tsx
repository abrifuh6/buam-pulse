import { useCallback, useEffect, useState } from 'react'
import { api, type Channel } from './api'

export default function Channels() {
  const [channels, setChannels] = useState<Channel[]>([])
  const [type, setType] = useState<'email' | 'slack'>('email')
  const [name, setName] = useState('')
  const [value, setValue] = useState('')
  const [err, setErr] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      setChannels(await api.listChannels())
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
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
          ? 'Verification email sent — confirm it before alerts are delivered.'
          : 'Channel added.',
      )
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
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
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }

  async function remove(id: string, label: string) {
    if (!confirm(`Delete channel "${label}"?`)) return
    await api.deleteChannel(id)
    load()
  }

  return (
    <>
      <div className="card">
        <div className="form-grid">
          <input
            placeholder="Channel name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <select value={type} onChange={(e) => setType(e.target.value as 'email' | 'slack')}>
            <option value="email">Email</option>
            <option value="slack">Slack</option>
          </select>
          <input
            placeholder={type === 'email' ? 'ops@yourcompany.com' : 'https://hooks.slack.com/...'}
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
          <span />
          <button onClick={add} disabled={busy || !name || !value}>
            Add
          </button>
        </div>
        {err && <p className="err">{err}</p>}
        {note && <p className="muted" style={{ marginTop: 10, fontSize: 13 }}>{note}</p>}
      </div>

      {!channels.length && (
        <p className="muted">
          No alert channels yet. Without one, Pulse detects outages but can't tell anyone.
        </p>
      )}

      {channels.map((c) => (
        <div className="card" key={c.id}>
          <div className="row">
            <span className={`dot ${c.verified ? 'up' : 'unknown'}`} />
            <div className="grow">
              <div className="name">{c.name}</div>
              <div className="target">
                {c.type === 'email' ? c.config.address : c.config.webhook_url}
                {!c.verified && ' · awaiting verification'}
              </div>
            </div>
            <button className="ghost" onClick={() => test(c.id)} disabled={!c.verified}>
              Test
            </button>
            <button className="ghost" onClick={() => remove(c.id, c.name)}>
              Delete
            </button>
          </div>
        </div>
      ))}
    </>
  )
}

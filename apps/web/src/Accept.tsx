import { useEffect, useState } from 'react'
import { api, token } from './api'

// Rendered when the URL is /accept?token=… — the link from an invitation email.
// On success it navigates to the dashboard rather than calling back into App,
// so no props are needed.
export default function Accept() {
  const inviteToken = new URLSearchParams(window.location.search).get('token') ?? ''
  const [info, setInfo] = useState<{ tenant: string; email: string; role: string } | null>(null)
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!inviteToken) {
      setErr('No invitation token in this link.')
      return
    }
    api
      .invitationInfo(inviteToken)
      .then(setInfo)
      .catch((e) => setErr(e instanceof Error ? e.message : 'failed'))
  }, [inviteToken])

  async function accept() {
    setErr('')
    setBusy(true)
    try {
      const res = await api.acceptInvitation(inviteToken, password)
      token.set(res.token)
      // Hand off to the dashboard with a full navigation. Using replaceState
      // here instead would re-run this component's effect against a URL that
      // no longer has a token, flashing a spurious error at the user.
      window.location.replace('/')
      return
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  if (err && !info)
    return (
      <div className="wrap">
        <div className="center card">
          <p className="err">{err}</p>
        </div>
      </div>
    )

  if (!info)
    return (
      <div className="wrap">
        <p className="center muted">Loading invitation…</p>
      </div>
    )

  return (
    <div className="wrap">
      <div className="center card">
        <h1 className="brand">
          Pulse <small>by Buam Technologies</small>
        </h1>
        <p className="muted" style={{ marginTop: 14, fontSize: 14 }}>
          You've been invited to join <strong>{info.tenant}</strong> as {info.role}.
        </p>
        <div className="stack" style={{ marginTop: 18 }}>
          <input value={info.email} disabled />
          <input
            placeholder="Choose a password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && accept()}
          />
          <button onClick={accept} disabled={busy || password.length < 8}>
            {busy ? '…' : 'Join team'}
          </button>
          {err && <p className="err">{err}</p>}
        </div>
      </div>
    </div>
  )
}

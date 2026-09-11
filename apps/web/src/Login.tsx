import { useState } from 'react'
import { api, token } from './api'

export default function Login({ onAuth }: { onAuth: () => void }) {
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [company, setCompany] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [sentReset, setSentReset] = useState(false)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit() {
    setErr('')
    setBusy(true)
    try {
      const res =
        mode === 'signup'
          ? await api.signup(company, email, password)
          : await api.login(email, password)
      token.set(res.token)
      onAuth()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Something went wrong. Try again.')
    } finally {
      setBusy(false)
    }
  }

  // Always reports success, matching the API: telling the user whether an
  // address exists would be an account-enumeration oracle.
  async function forgot() {
    if (!email) {
      setErr('Enter your email address first.')
      return
    }
    setErr('')
    setBusy(true)
    try {
      await api.forgotPassword(email)
      setSentReset(true)
    } catch {
      setSentReset(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth">
      <div className="auth-card">
        <h1 className="auth-brand">
          <span className="brand-mark">▚</span> Pulse
        </h1>
        <p className="auth-sub">
          {mode === 'signup'
            ? 'Know when your sites go down, before your customers tell you.'
            : 'Sign in to your monitoring dashboard.'}
        </p>

        <div className="panel">
          <div className="stack">
            {mode === 'signup' && (
              <div className="field">
                <label htmlFor="company">Company</label>
                <input
                  id="company"
                  placeholder="Northgate Digital"
                  value={company}
                  onChange={(e) => setCompany(e.target.value)}
                />
              </div>
            )}

            <div className="field">
              <label htmlFor="email">Email</label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>

            <div className="field">
              <label htmlFor="password">Password</label>
              <input
                id="password"
                type="password"
                autoComplete={mode === 'signup' ? 'new-password' : 'current-password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && submit()}
              />
              {mode === 'signup' && <span className="hint">At least 8 characters.</span>}
            </div>

            <button onClick={submit} disabled={busy || !email || !password}>
              {busy ? 'Working…' : mode === 'signup' ? 'Create account' : 'Sign in'}
            </button>

            {err && <p className="err">{err}</p>}
            {sentReset && (
              <p className="ok-note">
                If that address has an account, a reset link is on its way.
              </p>
            )}
          </div>
        </div>

        <div className="spread" style={{ marginTop: 'var(--s-3)' }}>
          <button
            className="quiet"
            onClick={() => {
              setMode(mode === 'login' ? 'signup' : 'login')
              setErr('')
              setSentReset(false)
            }}
          >
            {mode === 'login' ? 'Create an account' : 'Sign in instead'}
          </button>
          {mode === 'login' && (
            <button className="quiet" onClick={forgot} disabled={busy}>
              Forgot password
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

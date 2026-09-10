import { useState } from 'react'
import { api, token } from './api'

export default function Login({ onAuth }: { onAuth: () => void }) {
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [company, setCompany] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  async function handle() {
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
      setErr(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wrap">
      <div className="center card">
        <h1 className="brand">
          Pulse <small>by Buam Technologies</small>
        </h1>
        <div className="stack" style={{ marginTop: 18 }}>
          {mode === 'signup' && (
            <input
              placeholder="Company name"
              value={company}
              onChange={(e) => setCompany(e.target.value)}
            />
          )}
          <input
            placeholder="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <input
            placeholder="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handle()}
          />
          <button onClick={handle} disabled={busy}>
            {busy ? '…' : mode === 'signup' ? 'Create account' : 'Sign in'}
          </button>
          <button
            className="ghost"
            onClick={() => {
              setMode(mode === 'login' ? 'signup' : 'login')
              setErr('')
            }}
          >
            {mode === 'login' ? 'Create an account' : 'I already have an account'}
          </button>
          {err && <p className="err">{err}</p>}
        </div>
      </div>
    </div>
  )
}

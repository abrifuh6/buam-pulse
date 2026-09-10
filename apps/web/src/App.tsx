import { useState } from 'react'
import Login from './Login'
import Dashboard from './Dashboard'
import Accept from './Accept'
import { token } from './api'

export default function App() {
  const [authed, setAuthed] = useState(!!token.get())

  // Minimal routing: only one path needs special handling, so a full router
  // would be more machinery than the problem deserves.
  if (window.location.pathname === '/accept') {
    return <Accept />
  }

  return authed ? (
    <Dashboard onLogout={() => setAuthed(false)} />
  ) : (
    <Login onAuth={() => setAuthed(true)} />
  )
}

import { useState } from 'react'
import Login from './Login'
import Dashboard from './Dashboard'
import { token } from './api'

export default function App() {
  const [authed, setAuthed] = useState(!!token.get())
  return authed ? (
    <Dashboard onLogout={() => setAuthed(false)} />
  ) : (
    <Login onAuth={() => setAuthed(true)} />
  )
}

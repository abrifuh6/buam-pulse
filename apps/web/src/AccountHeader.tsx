import { useEffect, useRef, useState } from 'react'
import { token, type Account } from './api'

// Compact account control: avatar, name, plan badge, chevron, and a menu.
// Sits in the header as identity rather than as content.
export default function AccountHeader({
  account,
  onOpenPlan,
  onOpenTeam,
  onLogout,
}: {
  account: Account
  onOpenPlan: () => void
  onOpenTeam: () => void
  onLogout: () => void
}) {
  const [open, setOpen] = useState(false)
  const [copied, setCopied] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  // Close on outside click and on Escape — a menu that does neither feels broken.
  useEffect(() => {
    if (!open) return
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  async function copyStatusURL() {
    await navigator.clipboard.writeText(account.status_url)
    setCopied(true)
    setTimeout(() => setCopied(false), 1600)
  }

  const handle = account.user.email.split('@')[0]
  const initial = handle.charAt(0).toUpperCase()

  return (
    <div className="acct" ref={ref}>
      <button className="acct-btn" onClick={() => setOpen(!open)}>
        <span className="avatar">{initial}</span>
        <span className="acct-name">{handle}</span>
        <span className="acct-plan">· {account.plan.name}</span>
        <svg className="chev" width="12" height="12" viewBox="0 0 24 24" fill="none" aria-hidden>
          <path
            d="M6 9l6 6 6-6"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>

      {open && (
        <div className="acct-menu">
          <div className="acct-menu-head">
            <div className="acct-email">{account.tenant.name}</div>
            <div className="acct-role">
              {account.user.email} · {account.user.role}
            </div>
          </div>

          <button
            onClick={() => {
              onOpenPlan()
              setOpen(false)
            }}
          >
            Plan <span className="acct-meta">{account.plan.name}</span>
          </button>

          <button
            onClick={() => {
              onOpenTeam()
              setOpen(false)
            }}
          >
            Team{' '}
            <span className="acct-meta">
              {account.counts.members} {account.counts.members === 1 ? 'member' : 'members'}
            </span>
          </button>

          <button onClick={copyStatusURL}>
            {copied ? 'Link copied' : 'Copy status page link'}
          </button>

          <div className="acct-sep" />

          <button
            className="danger"
            onClick={() => {
              token.clear()
              onLogout()
            }}
          >
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}

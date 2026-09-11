import type { ReactNode } from 'react'
import AccountHeader from './AccountHeader'
import type { Account } from './api'

export type Tab = 'monitors' | 'channels' | 'maintenance' | 'status' | 'team' | 'billing'

// Icons are inline rather than a library: six 16px glyphs is not worth a
// dependency, and hand-drawn paths keep the stroke weight consistent with the
// rest of the interface.
function Icon({ name }: { name: Tab }) {
  const common = {
    width: 16,
    height: 16,
    viewBox: '0 0 16 16',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.5,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  }
  switch (name) {
    case 'monitors':
      return (
        <svg {...common}>
          <path d="M1 9h3l2-5 3 10 2-6h4" />
        </svg>
      )
    case 'channels':
      return (
        <svg {...common}>
          <path d="M8 2a4 4 0 0 0-4 4v3l-1.5 2.5h11L12 9V6a4 4 0 0 0-4-4z" />
          <path d="M6.5 14a1.5 1.5 0 0 0 3 0" />
        </svg>
      )
    case 'maintenance':
      return (
        <svg {...common}>
          <rect x="2" y="3" width="12" height="11" rx="1.5" />
          <path d="M2 6.5h12M5.5 1.5v3M10.5 1.5v3" />
        </svg>
      )
    case 'status':
      return (
        <svg {...common}>
          <circle cx="8" cy="8" r="6" />
          <path d="M8 4.5v3.5l2.5 1.5" />
        </svg>
      )
    case 'team':
      return (
        <svg {...common}>
          <circle cx="6" cy="6" r="2.5" />
          <path d="M1.5 13.5a4.5 4.5 0 0 1 9 0M11 4.2a2.5 2.5 0 0 1 0 4.6M12 13.5a4.5 4.5 0 0 0-1.2-3" />
        </svg>
      )
    case 'billing':
      return (
        <svg {...common}>
          <rect x="1.5" y="3.5" width="13" height="9" rx="1.5" />
          <path d="M1.5 7h13" />
        </svg>
      )
  }
}

const NAV: { id: Tab; label: string }[] = [
  { id: 'monitors', label: 'Monitors' },
  { id: 'channels', label: 'Alerts' },
  { id: 'maintenance', label: 'Maintenance' },
  { id: 'status', label: 'Status page' },
  { id: 'team', label: 'Team' },
  { id: 'billing', label: 'Plan' },
]

export default function Shell({
  tab,
  onTab,
  account,
  onLogout,
  downCount,
  title,
  subtitle,
  actions,
  children,
}: {
  tab: Tab
  onTab: (t: Tab) => void
  account: Account | null
  onLogout: () => void
  downCount: number
  title: string
  subtitle?: string
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="shell">
      <nav className="sidebar">
        <button className="brand" onClick={() => onTab('monitors')} aria-label="Go to monitors">
          <span className="brand-mark">▚</span> Pulse
        </button>

        <div className="nav">
          {NAV.map((n) => (
            <button
              key={n.id}
              className="nav-item"
              aria-current={tab === n.id ? 'page' : undefined}
              onClick={() => onTab(n.id)}
            >
              <Icon name={n.id} />
              {n.label}
              {/* The count of failing monitors follows you around the app: you
                  should never have to return to one screen to learn something
                  is broken. */}
              {n.id === 'monitors' && downCount > 0 && (
                <span className="nav-count">{downCount}</span>
              )}
            </button>
          ))}
        </div>

        <div className="sidebar-foot">
          {account && (
            <AccountHeader
              account={account}
              onOpenPlan={() => onTab('billing')}
              onOpenTeam={() => onTab('team')}
              onLogout={onLogout}
            />
          )}
        </div>
      </nav>

      <main className="main">
        <div className="main-inner">
          <header className="page-head">
            <div className="grow">
              <h1 className="page-title">{title}</h1>
              {subtitle && <p className="page-sub">{subtitle}</p>}
            </div>
            {actions}
          </header>
          {children}
        </div>
      </main>
    </div>
  )
}

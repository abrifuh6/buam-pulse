import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type Account, type Channel, type Monitor } from './api'
import Shell, { type Tab } from './Shell'
import { withTransition } from './transition'
import Monitors from './Monitors'
import Incidents from './Incidents'
import Channels from './Channels'
import Maintenance from './Maintenance'
import StatusPage from './StatusPage'
import Team from './Team'
import Billing from './Billing'
import AccountSettings from './AccountSettings'

const PAGE: Record<Tab, { title: string; subtitle: string }> = {
  monitors: { title: 'Monitors', subtitle: 'Every check you are running, and how it is behaving.' },
  incidents: { title: 'Incidents', subtitle: 'What has broken, for how long, and who was told.' },
  channels: { title: 'Alerts', subtitle: 'Where Pulse sends you when something breaks.' },
  maintenance: { title: 'Maintenance', subtitle: 'Planned work, with alerts suppressed.' },
  status: { title: 'Status page', subtitle: 'What your customers see.' },
  team: { title: 'Team', subtitle: 'Who can see and change what.' },
  billing: { title: 'Plan', subtitle: 'Usage against your limits.' },
  account: { title: 'Account', subtitle: 'Your password, your data, and leaving.' },
}

export default function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [tab, setTab] = useState<Tab>('monitors')
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [account, setAccount] = useState<Account | null>(null)
  const [loading, setLoading] = useState(true)
  // Set when an incident row is clicked, so the monitors screen opens straight
  // into that monitor's detail rather than the list.
  const [openMonitor, setOpenMonitor] = useState<string | null>(null)
  const [err, setErr] = useState('')

  const canWrite = currentRole() === 'owner' || currentRole() === 'admin'

  const load = useCallback(async () => {
    try {
      setMonitors(await api.listMonitors())
      setErr('')
    } catch (e) {
      if (e instanceof Error && e.message.startsWith('Your session')) return onLogout()
      setErr(e instanceof Error ? e.message : 'Could not load monitors.')
    } finally {
      setLoading(false)
    }
  }, [onLogout])

  useEffect(() => {
    api.account().then(setAccount).catch(() => {})
    api.listChannels().then(setChannels).catch(() => {})
  }, [])

  // Someone who chose a plan on the marketing site arrives with that intent
  // stored. This waits for the account to load before acting: firing on mount
  // races the auth check, and a checkout call without a session fails silently.
  //
  // Only the owner can start a checkout, and a brand-new signup always is one,
  // so this only ever runs for the person who made the choice.
  useEffect(() => {
    if (!account) return
    const raw = sessionStorage.getItem('pulse.intent')
    if (!raw) return
    sessionStorage.removeItem('pulse.intent')

    let plan = ''
    let period = 'monthly'
    try {
      const parsed = JSON.parse(raw) as { plan?: string; period?: string }
      plan = parsed.plan ?? ''
      period = parsed.period ?? 'monthly'
    } catch {
      return
    }
    if (!plan || plan === 'free' || plan === account.plan.code) return

    api
      .checkout(plan, period)
      .then(({ url }) => {
        window.location.href = url
      })
      .catch(() => {
        // Most likely they are not the owner. Land them on the plan screen
        // rather than silently discarding the choice they made.
        setTab('billing')
      })
  }, [account])

  // Monitors refresh on a timer because their state changes without the user
  // doing anything — that is the entire point of the product.
  useEffect(() => {
    load()
    const t = window.setInterval(load, 15000)
    return () => window.clearInterval(t)
  }, [load])

  const downCount = monitors.filter((m) => m.enabled && m.status === 'down').length

  return (
    <Shell
      tab={tab}
      onTab={(t) => withTransition(() => setTab(t))}
      account={account}
      onLogout={onLogout}
      downCount={downCount}
      title={PAGE[tab].title}
      subtitle={PAGE[tab].subtitle}
    >
      {err && <p className="err">{err}</p>}

      {tab === 'incidents' && (
        <Incidents
          onOpenMonitor={(id) =>
            withTransition(() => {
              setTab('monitors')
              setOpenMonitor(id)
            })
          }
        />
      )}
      {tab === 'monitors' && (
        <Monitors
          monitors={monitors}
          channels={channels}
          canWrite={canWrite}
          loading={loading}
          openMonitor={openMonitor}
          onOpened={() => setOpenMonitor(null)}
          onChanged={load}
        />
      )}
      {tab === 'channels' && <Channels />}
      {tab === 'maintenance' && <Maintenance monitors={monitors} />}
      {tab === 'status' && <StatusPage monitors={monitors} onChanged={load} />}
      {tab === 'team' && <Team />}
      {tab === 'billing' && <Billing account={account} />}
      {tab === 'account' && <AccountSettings account={account} onLogout={onLogout} />}
    </Shell>
  )
}

import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type Account, type Plan, type PlanUsage } from './api'

function money(cents: number): string {
  return cents === 0 ? 'Free' : `$${(cents / 100).toFixed(0)}/mo`
}

function interval(seconds: number): string {
  return seconds >= 60 ? `${seconds / 60} min` : `${seconds}s`
}

// The bar turns amber at 80% so a limit is visible before it is hit, not after
// a creation has already been refused.
function Usage({ label, used, cap }: { label: string; used: number; cap: number }) {
  const pct = Math.min(100, (used / cap) * 100)
  const state = pct >= 100 ? 'bad' : pct >= 80 ? 'warn' : 'ok'
  return (
    <div style={{ marginBottom: 'var(--s-4)' }}>
      <div className="spread" style={{ marginBottom: 'var(--s-2)' }}>
        <span>{label}</span>
        <span className="num">
          {used} / {cap}
        </span>
      </div>
      <div className="meter">
        <i className={state} style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}

export default function Billing({ account }: { account: Account | null }) {
  const isOwner = currentRole() === 'owner'

  const [current, setCurrent] = useState<PlanUsage | null>(null)
  const [all, setAll] = useState<Plan[]>([])
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState('')

  const load = useCallback(() => {
    Promise.all([api.currentPlan(), api.listPlans()])
      .then(([c, a]) => {
        setCurrent(c)
        setAll(a)
      })
      .catch((e) => setErr(e instanceof Error ? e.message : 'Could not load your plan.'))
  }, [])

  useEffect(load, [load])

  // A tenant with no Stripe customer has never paid, so there is nothing for the
  // portal to manage and they go through checkout. Everyone else goes to the
  // portal, which handles upgrades, downgrades, cards and invoices in one place.
  // Rebuilding that ourselves would mean implementing proration and refunds.
  async function choose(planCode: string) {
    setErr('')
    setBusy(planCode)
    try {
      const { url } = account?.plan.has_billing
        ? await api.billingPortal()
        : await api.checkout(planCode)
      window.location.href = url
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not open billing.')
      setBusy('')
    }
  }

  async function manage() {
    setErr('')
    setBusy('portal')
    try {
      const { url } = await api.billingPortal()
      window.location.href = url
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not open billing.')
      setBusy('')
    }
  }

  if (!current) return <div className="skeleton" style={{ height: 300 }} />

  const pastDue = account?.plan.subscription_status === 'past_due'
  const ending = account?.plan.cancel_at_period_end

  return (
    <>
      {pastDue && (
        <div className="panel" style={{ borderColor: 'var(--down)' }}>
          <div className="spread">
            <div>
              <h2 className="panel-title" style={{ color: 'var(--down)' }}>
                Your last payment failed
              </h2>
              <p className="panel-note">
                Update your card to keep {current.plan.name}. Stripe will retry for a few days
                before the plan drops.
              </p>
            </div>
            {isOwner && <button onClick={manage}>Update card</button>}
          </div>
        </div>
      )}

      {ending && !pastDue && (
        <div className="panel">
          <p className="panel-note" style={{ margin: 0 }}>
            Your subscription ends when the current billing period closes. You keep{' '}
            {current.plan.name} until then.
          </p>
        </div>
      )}

      <div className="panel">
        <div className="spread" style={{ marginBottom: 'var(--s-5)' }}>
          <div>
            <h2 className="panel-title">{current.plan.name}</h2>
            <p className="panel-note">
              {money(current.plan.price_cents)}, checks as often as every{' '}
              {interval(current.plan.min_interval)}, {current.plan.retention_days} days of history
            </p>
          </div>
          {isOwner && account?.plan.has_billing && (
            <button className="ghost" onClick={manage} disabled={busy !== ''}>
              {busy === 'portal' ? 'Opening…' : 'Manage billing'}
            </button>
          )}
        </div>

        <Usage label="Monitors" used={current.usage.monitors} cap={current.plan.max_monitors} />
        <Usage label="Team members" used={current.usage.members} cap={current.plan.max_members} />
        <Usage label="Alert channels" used={current.usage.channels} cap={current.plan.max_channels} />
      </div>

      {err && <p className="err">{err}</p>}

      <p className="section-label">Plans</p>

      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr>
              <th>Plan</th>
              <th className="right">Monitors</th>
              <th className="right">Fastest check</th>
              <th className="right">Members</th>
              <th className="right">History</th>
              <th className="right">Price</th>
              <th className="right" style={{ width: 120 }} />
            </tr>
          </thead>
          <tbody>
            {all.map((p) => {
              const isCurrent = p.code === current.plan.code
              const isUpgrade = p.price_cents > current.plan.price_cents
              return (
                <tr key={p.code}>
                  <td>
                    <div className="cell-name">
                      {p.name}
                      {isCurrent && (
                        <span className="badge up" style={{ marginLeft: 8 }}>
                          current
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="right num">{p.max_monitors}</td>
                  <td className="right num">{interval(p.min_interval)}</td>
                  <td className="right num">{p.max_members}</td>
                  <td className="right num">{p.retention_days}d</td>
                  <td className="right num">{money(p.price_cents)}</td>
                  <td className="right">
                    {!isCurrent && isOwner && (
                      <button
                        className={isUpgrade ? '' : 'ghost'}
                        onClick={() => choose(p.code)}
                        disabled={busy !== ''}
                      >
                        {busy === p.code ? '…' : isUpgrade ? 'Upgrade' : 'Downgrade'}
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {!isOwner && (
        <p className="metric" style={{ marginTop: 'var(--s-4)' }}>
          Only the account owner can change the plan.
        </p>
      )}
    </>
  )
}

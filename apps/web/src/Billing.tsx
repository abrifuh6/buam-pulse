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

      <div className="plan-grid">
        {all.map((p) => {
          const isCurrent = p.code === current.plan.code
          const isUpgrade = p.price_cents > current.plan.price_cents
          // The middle tier carries the recommendation. Real pricing pages do
          // this because an unanchored set of options makes people choose
          // nothing; a marked default gives the decision a starting point.
          const recommended = p.code === 'starter' && !isCurrent

          return (
            <div
              key={p.code}
              className={`plan ${isCurrent ? 'is-current' : ''} ${
                recommended ? 'is-recommended' : ''
              }`}
            >
              {recommended && <span className="plan-flag">Most teams start here</span>}
              {isCurrent && <span className="plan-flag current">Your plan</span>}

              <h3 className="plan-name">{p.name}</h3>
              <div className="plan-price">
                {p.price_cents === 0 ? (
                  <b>Free</b>
                ) : (
                  <>
                    <b>${(p.price_cents / 100).toFixed(0)}</b>
                    <span>per month</span>
                  </>
                )}
              </div>

              <ul className="plan-features">
                <li>
                  <b className="num">{p.max_monitors}</b> monitors
                </li>
                <li>
                  checks every <b className="num">{interval(p.min_interval)}</b>
                </li>
                <li>
                  <b className="num">{p.max_members}</b>{' '}
                  {p.max_members === 1 ? 'team member' : 'team members'}
                </li>
                <li>
                  <b className="num">{p.max_channels}</b> alert{' '}
                  {p.max_channels === 1 ? 'channel' : 'channels'}
                </li>
                <li>
                  <b className="num">{p.retention_days}</b> days of history
                </li>
              </ul>

              {isCurrent ? (
                <button className="ghost" disabled>
                  Current plan
                </button>
              ) : isOwner ? (
                <button
                  className={isUpgrade ? '' : 'ghost'}
                  onClick={() => choose(p.code)}
                  disabled={busy !== ''}
                >
                  {busy === p.code ? 'Opening…' : isUpgrade ? `Upgrade to ${p.name}` : `Switch to ${p.name}`}
                </button>
              ) : (
                <button className="ghost" disabled>
                  Owner only
                </button>
              )}
            </div>
          )
        })}
      </div>

      {!isOwner && (
        <p className="metric" style={{ marginTop: 'var(--s-4)' }}>
          Only the account owner can change the plan.
        </p>
      )}
    </>
  )
}

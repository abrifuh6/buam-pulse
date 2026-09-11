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
  // Defaults to whatever the tenant is already billed on, so the screen opens
  // showing their actual price rather than a monthly figure they don't pay.
  const [period, setPeriod] = useState<Period>(
    (account?.plan.billing_period as Period) ?? 'monthly',
  )
  // Which card the user is looking at. Starts on their own plan; clicking
  // another previews it. Distinct from what they are actually on, which never
  // changes without going through Stripe.
  const [focused, setFocused] = useState<string | null>(null)

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
        : await api.checkout(planCode, period)
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

      {account?.plan.trial_ends_at &&
        account.plan.subscription_status === 'trialing' && (
          <div className="panel" style={{ borderColor: 'var(--accent)' }}>
            <p className="panel-note" style={{ margin: 0 }}>
              Your {current.plan.name} trial runs until{' '}
              {new Date(account.plan.trial_ends_at).toLocaleDateString(undefined, {
                month: 'long',
                day: 'numeric',
              })}
              . Your card is charged then unless you cancel before.
            </p>
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

      <div className="spread" style={{ margin: 'var(--s-6) 0 var(--s-4)' }}>
        <p className="section-label" style={{ margin: 0 }}>
          Plans
        </p>
        <div className="period-toggle">
          {PERIODS.map((pr) => (
            <button
              key={pr.id}
              className={period === pr.id ? 'on' : ''}
              onClick={() => setPeriod(pr.id)}
            >
              {pr.label}
              {pr.save && <span className="save">{pr.save}</span>}
            </button>
          ))}
        </div>
      </div>

      <div className="plan-grid">
        {all.map((p) => {
          const isCurrent = p.code === current.plan.code
          const total = priceFor(p, period)
          const months = PERIODS.find((x) => x.id === period)!.months
          const perMonth = total / months / 100
          const isFree = total === 0
          const isUpgrade = total > priceFor(current.plan, period)

          const isFocused = focused === p.code

          return (
            <div
              key={p.code}
              role="radio"
              aria-checked={isFocused || (focused === null && isCurrent)}
              tabIndex={0}
              className={`plan ${isCurrent ? 'is-current' : ''} ${
                isFocused && !isCurrent ? 'is-focused' : ''
              }`}
              onClick={() => setFocused(p.code)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  setFocused(p.code)
                }
              }}
            >
              {/* Only one highlight on this screen. The tenant's own plan is
                  the thing that matters here, so a generic recommendation is
                  not shown alongside it — two competing markers means neither
                  reads as authoritative. */}
              {isCurrent ? (
                <span className="plan-flag current">Your plan</span>
              ) : isFocused ? (
                <span className="plan-flag">
                  {isUpgrade ? 'Upgrading to this' : 'Switching to this'}
                </span>
              ) : null}

              <h3 className="plan-name">{p.name}</h3>

              <div>
                <div className="plan-price">
                  {isFree ? (
                    <b>Free</b>
                  ) : (
                    <>
                      <b>${perMonth % 1 === 0 ? perMonth : perMonth.toFixed(2)}</b>
                      <span>per month</span>
                    </>
                  )}
                </div>
                {!isFree && period !== 'monthly' && (
                  <div className="metric" style={{ marginTop: 'var(--s-1)' }}>
                    ${(total / 100).toFixed(0)} billed{' '}
                    {period === 'yearly' ? 'yearly' : 'every 3 months'}
                  </div>
                )}
              </div>

              {isFocused && !isCurrent && (
                <div className="plan-delta">
                  {current.usage.monitors > p.max_monitors ? (
                    <span style={{ color: 'var(--warn)' }}>
                      You have {current.usage.monitors} monitors. On {p.name} you keep them all,
                      but cannot add more until you are under {p.max_monitors}.
                    </span>
                  ) : current.usage.members > p.max_members ? (
                    <span style={{ color: 'var(--warn)' }}>
                      You have {current.usage.members} team members, more than {p.name} allows.
                    </span>
                  ) : (
                    <span>Everything you have today fits on this plan.</span>
                  )}
                </div>
              )}

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
                  className={isFocused ? '' : 'ghost'}
                  onClick={(e) => {
                    e.stopPropagation()
                    choose(p.code)
                  }}
                  disabled={busy !== ''}
                >
                  {busy === p.code
                    ? 'Opening…'
                    : account?.plan.has_billing
                      ? `Switch to ${p.name}`
                      : isFree
                        ? 'Switch to Free'
                        : `Try ${p.name} free for 14 days`}
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

type Period = 'monthly' | 'quarterly' | 'yearly'

const PERIODS: { id: Period; label: string; months: number; save?: string }[] = [
  { id: 'monthly', label: 'Monthly', months: 1 },
  { id: 'quarterly', label: 'Quarterly', months: 3, save: '10% off' },
  { id: 'yearly', label: 'Yearly', months: 12, save: '20% off' },
]

// Falls back to the monthly price when a plan has no price for the chosen
// period, which is the case for Free — it has exactly one price and it is zero.
function priceFor(p: Plan, period: Period): number {
  if (period === 'quarterly') return p.price_cents_quarterly ?? p.price_cents * 3
  if (period === 'yearly') return p.price_cents_yearly ?? p.price_cents * 12
  return p.price_cents
}

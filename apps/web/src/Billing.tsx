import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type Account, type Plan, type PlanUsage } from './api'

function money(cents: number): string {
  return cents === 0 ? 'Free' : `$${(cents / 100).toFixed(0)}/mo`
}

function interval(seconds: number): string {
  return seconds >= 60 ? `${seconds / 60} min` : `${seconds}s`
}

function UsageRow({ label, used, cap }: { label: string; used: number; cap: number }) {
  const pct = Math.min(100, (used / cap) * 100)
  const state = pct >= 100 ? 'bad' : pct >= 80 ? 'warn' : 'ok'
  return (
    <div style={{ marginBottom: 14 }}>
      <div className="row" style={{ marginBottom: 6 }}>
        <span className="grow">{label}</span>
        <span className="metric">
          {used} of {cap}
        </span>
      </div>
      <div className="usage">
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
      .catch((e) => setErr(e instanceof Error ? e.message : 'failed'))
  }, [])

  useEffect(load, [load])

  // A tenant with no Stripe customer has never paid, so there is nothing for
  // the portal to manage — they go through Checkout. Everyone else goes to the
  // portal, which handles upgrades, downgrades, cancellation, card changes and
  // invoices in one place. Building those flows ourselves would duplicate a
  // lot of edge-case handling (proration, failed cards, tax) for no benefit.
  async function choose(planCode: string) {
    setErr('')
    setBusy(planCode)
    try {
      if (account?.plan.has_billing) {
        const { url } = await api.billingPortal()
        window.location.href = url
      } else {
        const { url } = await api.checkout(planCode)
        window.location.href = url
      }
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
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
      setErr(e instanceof Error ? e.message : 'failed')
      setBusy('')
    }
  }

  if (err && !current) return <p className="err">{err}</p>
  if (!current) return <p className="muted">Loading…</p>

  const pastDue = account?.plan.subscription_status === 'past_due'

  return (
    <>
      {account?.plan.cancel_at_period_end && (
        <div className="card" style={{ borderColor: 'var(--muted)' }}>
          <span className="muted">
            Your subscription is set to end at the close of the current billing
            period. You keep {current.plan.name} access until then.
          </span>
        </div>
      )}

      {pastDue && (
        <div className="card" style={{ borderColor: 'var(--down)' }}>
          <div className="row">
            <span className="grow" style={{ color: 'var(--down)' }}>
              Your last payment failed. Update your card to avoid losing your plan.
            </span>
            {isOwner && (
              <button onClick={manage} disabled={busy !== ''}>
                Update payment
              </button>
            )}
          </div>
        </div>
      )}

      <div className="card">
        <div className="row" style={{ marginBottom: 18 }}>
          <div className="grow">
            <div className="name">{current.plan.name} plan</div>
            <div className="target">
              {money(current.plan.price_cents)} · checks as often as every{' '}
              {interval(current.plan.min_interval)} · {current.plan.retention_days} days of
              history
            </div>
          </div>
          {isOwner && account?.plan.has_billing && (
            <button className="ghost" onClick={manage} disabled={busy !== ''}>
              {busy === 'portal' ? '…' : 'Manage billing'}
            </button>
          )}
        </div>

        <UsageRow label="Monitors" used={current.usage.monitors} cap={current.plan.max_monitors} />
        <UsageRow label="Team members" used={current.usage.members} cap={current.plan.max_members} />
        <UsageRow
          label="Alert channels"
          used={current.usage.channels}
          cap={current.plan.max_channels}
        />
      </div>

      {err && <p className="err">{err}</p>}

      <p className="muted" style={{ marginTop: 24, fontSize: 13 }}>
        Plans
      </p>

      {all.map((p) => {
        const isCurrent = p.code === current.plan.code
        const isUpgrade = p.price_cents > current.plan.price_cents
        return (
          <div className="card" key={p.code} style={{ opacity: isCurrent ? 1 : 0.85 }}>
            <div className="row">
              <div className="grow">
                <div className="name">
                  {p.name}
                  {isCurrent && <span className="metric"> · current</span>}
                </div>
                <div className="target">
                  {p.max_monitors} monitors · {interval(p.min_interval)} minimum ·{' '}
                  {p.max_members} {p.max_members === 1 ? 'member' : 'members'} ·{' '}
                  {p.retention_days}d history
                </div>
              </div>
              <span className="metric" style={{ fontSize: 15 }}>
                {money(p.price_cents)}
              </span>
              {!isCurrent && isOwner && (
                <button
                  className={isUpgrade ? '' : 'ghost'}
                  onClick={() => choose(p.code)}
                  disabled={busy !== ''}
                >
                  {busy === p.code ? '…' : isUpgrade ? 'Upgrade' : 'Downgrade'}
                </button>
              )}
            </div>
          </div>
        )
      })}

      {!isOwner && (
        <p className="muted" style={{ marginTop: 14, fontSize: 13 }}>
          Only the account owner can change the plan.
        </p>
      )}
    </>
  )
}

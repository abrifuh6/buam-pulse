import { useEffect, useState } from 'react'
import { api, currentRole, type Plan, type PlanUsage } from './api'

function money(cents: number): string {
  return cents === 0 ? 'Free' : `$${(cents / 100).toFixed(0)}/mo`
}

function interval(seconds: number): string {
  return seconds >= 60 ? `${seconds / 60} min` : `${seconds}s`
}

// A usage row shows consumption against the cap. The bar turns amber at 80%
// so the limit is visible before it's hit, not after.
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

export default function Billing() {
  const isOwner = currentRole() === 'owner'
  const [current, setCurrent] = useState<PlanUsage | null>(null)
  const [all, setAll] = useState<Plan[]>([])
  const [err, setErr] = useState('')

  useEffect(() => {
    Promise.all([api.currentPlan(), api.listPlans()])
      .then(([c, a]) => {
        setCurrent(c)
        setAll(a)
      })
      .catch((e) => setErr(e instanceof Error ? e.message : 'failed'))
  }, [])

  if (err) return <p className="err">{err}</p>
  if (!current) return <p className="muted">Loading…</p>

  return (
    <>
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
        </div>

        <UsageRow label="Monitors" used={current.usage.monitors} cap={current.plan.max_monitors} />
        <UsageRow label="Team members" used={current.usage.members} cap={current.plan.max_members} />
        <UsageRow
          label="Alert channels"
          used={current.usage.channels}
          cap={current.plan.max_channels}
        />
      </div>

      <p className="muted" style={{ marginTop: 24, fontSize: 13 }}>
        Plans
      </p>

      {all.map((p) => {
        const isCurrent = p.code === current.plan.code
        return (
          <div className="card" key={p.code} style={{ opacity: isCurrent ? 1 : 0.8 }}>
            <div className="row">
              <div className="grow">
                <div className="name">
                  {p.name}
                  {isCurrent && <span className="metric"> · current</span>}
                </div>
                <div className="target">
                  {p.max_monitors} monitors · {interval(p.min_interval)} minimum ·{' '}
                  {p.max_members} {p.max_members === 1 ? 'member' : 'members'} · {p.retention_days}d history
                </div>
              </div>
              <span className="metric" style={{ fontSize: 15 }}>
                {money(p.price_cents)}
              </span>
              {!isCurrent && isOwner && (
                <button className="ghost" disabled title="Billing is not connected yet">
                  {p.price_cents > current.plan.price_cents ? 'Upgrade' : 'Downgrade'}
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

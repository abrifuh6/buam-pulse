import { useEffect, useState } from 'react'
import HeroDemo from './HeroDemo'

// Where the dashboard lives. In production these are one domain; in dev they
// are separate ports.
const APP_URL = import.meta.env.VITE_APP_URL ?? 'http://localhost:5173'

export default function App() {
  const [scrolled, setScrolled] = useState(false)

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8)
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  return (
    <>
      <nav className={`nav ${scrolled ? 'scrolled' : ''}`}>
        <div className="wrap nav-inner">
          <a className="logo" href="/">
            <i>▚</i> Pulse
          </a>
          <div className="nav-links">
            <a href="#what">What it does</a>
            <a href="#how">How it works</a>
            <a href="#pricing">Pricing</a>
          </div>
          <div className="nav-end">
            <a className="btn ghost" href={APP_URL}>
              Sign in
            </a>
            <a className="btn" href={APP_URL}>
              Start free
            </a>
          </div>
        </div>
      </nav>

      <header className="hero">
        <div className="wrap hero-inner">
          <span className="hero-eyebrow">
            <span className="beat" />
            Checking 1,284 endpoints right now
          </span>

          <h1>
            Know before your
            <br />
            customers do.
          </h1>

          <p className="lead">
            Pulse watches your sites and APIs from the outside, every thirty seconds. When
            something breaks it tells you in under a minute — and tells your customers on a
            status page you do not have to write.
          </p>

          <div className="hero-cta">
            <a className="btn lg" href={APP_URL}>
              Start monitoring free
            </a>
            <a className="btn ghost lg" href="#how">
              See how a check works
            </a>
          </div>
          <p className="hero-note">Three monitors free, forever. No card required.</p>

          <HeroDemo />
        </div>
      </header>

      {/* The problem, stated as an argument rather than a feature list. */}
      <section id="what">
        <div className="wrap argument-grid">
          <p className="argument">Most outages are found by the customer, not the team.</p>
          <div className="argument-body">
            <p style={{ marginBottom: 'var(--s-4)' }}>
              Your server is up. The process is running. The health check returns 200. And yet
              checkout has been throwing an error for forty minutes, and the first you hear
              about it is a support ticket.
            </p>
            <p style={{ marginBottom: 'var(--s-4)' }}>
              Internal monitoring tells you what your infrastructure thinks. Pulse tells you
              what a visitor actually experiences: from outside your network, over real DNS and
              a real TLS handshake, checking that the page contains what it should.
            </p>
            <p>
              When something fails twice in a row, you get an email or a Slack message naming
              the service, the reason, and how long it has been down. Your status page updates
              itself.
            </p>
          </div>
        </div>
      </section>

      <section>
        <div className="wrap">
          <div className="section-head narrow">
            <h2>Everything a small team needs, and nothing it does not.</h2>
            <p className="lead">
              Pulse is not an observability platform. It answers one question well: is this
              working, and if not, for how long.
            </p>
          </div>

          <div className="features">
            <Feature
              mark="▚"
              title="Checks that catch real failures"
              body="HTTP and TCP, every 30 seconds. Assert the page contains the text it should — a 200 response with an error rendered inside it is still an outage, and status codes alone never catch it."
            />
            <Feature
              mark="◈"
              title="Alerts that do not cry wolf"
              body="Two consecutive failures before anything fires, an optional delay so a brief blip never wakes anyone, and maintenance windows that keep deploys quiet. Recovery is announced too."
            />
            <Feature
              mark="◷"
              title="Status pages, already written"
              body="A public page per account with 90 days of uptime history, live incidents and your own branding. Choose which services appear — internal checks stay private."
            />
            <Feature
              mark="⌁"
              title="Certificates before they expire"
              body="Every HTTPS check reads the certificate. You get a warning two weeks out, which is two weeks more than you get from the browser error your customers see first."
            />
            <Feature
              mark="◎"
              title="Latency you can actually read"
              body="Median and 95th percentile, not an average that hides the slow tail. Every check opens a fresh connection, so the number is what a first-time visitor experiences."
            />
            <Feature
              mark="⊞"
              title="A team, with real permissions"
              body="Invite people as admins or read-only members. Ownership transfers cleanly. Nobody has to share a login to see whether the site is up."
            />
          </div>
        </div>
      </section>

      {/* Genuinely a sequence, so the numbers are information not decoration. */}
      <section id="how">
        <div className="wrap">
          <div className="section-head narrow">
            <h2>What happens in thirty seconds.</h2>
            <p className="lead">
              A check is not a ping. Here is the whole path, every time.
            </p>
          </div>

          <div className="steps">
            <Step
              title="Resolve and connect"
              body="DNS is resolved fresh and the TLS handshake is performed on a new connection, so the timing reflects a cold visit rather than a warm pool."
            />
            <Step
              title="Assert"
              body="Status code, response body, and certificate expiry are all checked. Any one of them failing is a failure."
            />
            <Step
              title="Confirm"
              body="One failure is noise. Pulse waits for a second consecutive failure before opening an incident, and honours any delay you set."
            />
            <Step
              title="Tell everyone"
              body="Email and Slack within a minute, with retries if delivery fails. Your status page updates at the same moment."
            />
          </div>
        </div>
      </section>

      <Pricing appUrl={APP_URL} />

      <section className="close">
        <div className="wrap narrow">
          <h2>Find out yourself, next time.</h2>
          <p className="lead">
            Add your first monitor in about a minute. If nothing ever breaks, you will never
            hear from us.
          </p>
          <a className="btn lg" href={APP_URL}>
            Start monitoring free
          </a>
        </div>
      </section>

      <footer>
        <div className="wrap footer-inner">
          <span>Pulse — built by Buam Technologies Inc.</span>
          <div className="footer-links">
            <a href="#what">What it does</a>
            <a href="#how">How it works</a>
            <a href="#pricing">Pricing</a>
            <a href={APP_URL}>Sign in</a>
          </div>
        </div>
      </footer>
    </>
  )
}

function Feature({ mark, title, body }: { mark: string; title: string; body: string }) {
  return (
    <div className="feature">
      <span className="mark">{mark}</span>
      <h3>{title}</h3>
      <p>{body}</p>
    </div>
  )
}

function Step({ title, body }: { title: string; body: string }) {
  return (
    <div className="step">
      <h3>{title}</h3>
      <p>{body}</p>
    </div>
  )
}

type Period = 'monthly' | 'quarterly' | 'yearly'

const PLANS = [
  {
    code: 'free',
    name: 'Free',
    prices: { monthly: 0, quarterly: 0, yearly: 0 },
    features: [
      ['3', 'monitors'],
      ['5 min', 'minimum interval'],
      ['1', 'team member'],
      ['30 days', 'of history'],
    ] as [string, string][],
  },
  {
    code: 'starter',
    name: 'Starter',
    featured: true,
    prices: { monthly: 1900, quarterly: 5100, yearly: 18200 },
    features: [
      ['25', 'monitors'],
      ['1 min', 'minimum interval'],
      ['5', 'team members'],
      ['90 days', 'of history'],
    ] as [string, string][],
  },
  {
    code: 'pro',
    name: 'Pro',
    prices: { monthly: 4900, quarterly: 13200, yearly: 47000 },
    features: [
      ['100', 'monitors'],
      ['30 sec', 'minimum interval'],
      ['25', 'team members'],
      ['365 days', 'of history'],
    ] as [string, string][],
  },
]

const PERIODS: { id: Period; label: string; months: number; save?: string }[] = [
  { id: 'monthly', label: 'Monthly', months: 1 },
  { id: 'quarterly', label: 'Quarterly', months: 3, save: 'save 10%' },
  { id: 'yearly', label: 'Yearly', months: 12, save: 'save 20%' },
]

function Pricing({ appUrl }: { appUrl: string }) {
  const [period, setPeriod] = useState<Period>('monthly')
  // Starter is pre-selected as the recommendation rather than left to the
  // visitor: an unanchored set of options makes people choose nothing. They can
  // change it, and the card shows which one is theirs once they do.
  const [chosen, setChosen] = useState('starter')
  const months = PERIODS.find((p) => p.id === period)!.months

  return (
    <section id="pricing">
      <div className="wrap">
        <div className="section-head narrow">
          <h2>Pricing that stays out of the way.</h2>
          <p className="lead">
            Every plan includes alerting, status pages, certificate warnings and the full API.
            You are paying for scale, not for features being held back.
          </p>
        </div>

        {/* Prices are always shown per month regardless of billing period, so
            the comparison across the toggle is like for like. Showing $182 next
            to $19 would make the cheaper option look more expensive. */}
        <div className="period-toggle">
          {PERIODS.map((p) => (
            <button
              key={p.id}
              className={period === p.id ? 'on' : ''}
              onClick={() => setPeriod(p.id)}
            >
              {p.label}
              {p.save && <span className="save">{p.save}</span>}
            </button>
          ))}
        </div>

        <div className="plans" role="radiogroup" aria-label="Choose a plan">
          {PLANS.map((plan) => {
            const total = plan.prices[period]
            const perMonth = total / months / 100
            const isFree = total === 0
            const selected = chosen === plan.code

            return (
              <div
                key={plan.code}
                role="radio"
                aria-checked={selected}
                tabIndex={0}
                className={`plan ${selected ? 'featured' : ''}`}
                onClick={() => setChosen(plan.code)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    setChosen(plan.code)
                  }
                }}
              >
                {selected && (
                  <span className="plan-tag">
                    {plan.featured ? 'Most teams start here' : 'Selected'}
                  </span>
                )}
                <h3>{plan.name}</h3>

                <div>
                  <div className="price">
                    {isFree ? '$0' : `$${perMonth % 1 === 0 ? perMonth : perMonth.toFixed(2)}`}
                    <small>per month</small>
                  </div>
                  {!isFree && period !== 'monthly' && (
                    <div className="price-note">
                      ${(total / 100).toFixed(0)} billed {period === 'yearly' ? 'yearly' : 'every 3 months'}
                    </div>
                  )}
                  {isFree && <div className="price-note">free forever</div>}
                </div>

                <ul>
                  {plan.features.map(([value, label]) => (
                    <li key={label}>
                      <b>{value}</b> {label}
                    </li>
                  ))}
                </ul>

                {/* The chosen plan and period travel to the app as query
                    parameters, so someone who picked Pro yearly here lands in
                    Pro yearly checkout rather than a generic signup. */}
                <a
                  className={`btn ${plan.featured ? '' : 'ghost'}`}
                  href={
                    isFree
                      ? appUrl
                      : `${appUrl}/?plan=${plan.code}&period=${period}`
                  }
                >
                  {isFree ? 'Start free' : `Try ${plan.name} free for 14 days`}
                </a>
                {!isFree && <span className="plan-foot">Card required, cancel any time</span>}
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}

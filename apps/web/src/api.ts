// One place that knows how to talk to the API. Every call goes to a relative
// /api path, so the browser treats it as same-origin.

export type Monitor = {
  id: string
  name: string
  type: 'http' | 'tcp'
  target: string
  interval_seconds: number
  timeout_seconds: number
  expected_status?: number
  enabled: boolean
  status: 'up' | 'down' | 'unknown'
  created_at: string
  keyword: string | null
  keyword_present: boolean
  check_ssl: boolean
  ssl_warn_days: number
  ssl_expires_at: string | null
  ssl_issuer: string | null
  alert_delay_seconds: number
  public: boolean
  public_name: string | null
}

export type CheckResult = {
  checked_at: string
  ok: boolean
  status_code: number | null
  latency_ms: number | null
  error: string | null
  region: string
}


export type Channel = {
  id: string
  name: string
  type: 'email' | 'slack'
  config: Record<string, string>
  enabled: boolean
  verified: boolean
}


export type Member = {
  id: string
  email: string
  role: 'owner' | 'admin' | 'member'
  verified: boolean
  joined_at: string
  is_you: boolean
}

export type Invitation = {
  id: string
  email: string
  role: string
  expires_at: string
}


export type Plan = {
  code: string
  name: string
  max_monitors: number
  min_interval: number
  max_members: number
  max_channels: number
  retention_days: number
  price_cents: number
}

export type PlanUsage = {
  plan: Plan
  usage: { monitors: number; members: number; channels: number }
}


export type Account = {
  tenant: { name: string; slug: string }
  user: { email: string; role: 'owner' | 'admin' | 'member' }
  plan: {
    code: string
    name: string
    has_billing: boolean
    subscription_status: string | null
    cancel_at_period_end: boolean
  }
  counts: { members: number; monitors: number }
  status_url: string
}


export type MaintenanceWindow = {
  id: string
  title: string
  description: string | null
  starts_at: string
  ends_at: string
  monitor_ids: string[]
  active: boolean
}


export type StatusPageSettings = {
  title: string | null
  description: string | null
  support_url: string | null
  hide_branding: boolean
  can_hide_branding: boolean
  slug: string
  public_url: string
}


export type SeriesPoint = {
  at: string
  uptime: number
  latency_p50: number | null
  latency_p95: number | null
  checks: number
}

export type DetailIncident = {
  started_at: string
  resolved_at: string | null
  minutes: number
  planned: boolean
  notified: boolean
  cause: string | null
}

export type MonitorDetail = {
  uptime_24h: number
  uptime_7d: number
  uptime_30d: number
  avg_latency_ms: number | null
  p95_latency_ms: number | null
  series: SeriesPoint[]
  incidents: DetailIncident[]
}


export type IncidentRow = {
  id: string
  monitor: string
  monitor_id: string
  started_at: string
  resolved_at: string | null
  minutes: number
  planned: boolean
  notified: boolean
  cause: string | null
}

const TOKEN_KEY = 'pulse.token'

export const token = {
  get: () => localStorage.getItem(TOKEN_KEY),
  set: (t: string) => localStorage.setItem(TOKEN_KEY, t),
  clear: () => localStorage.removeItem(TOKEN_KEY),
}


// The JWT payload carries the role. Reading it client-side is only for showing
// or hiding UI — the server enforces permissions regardless of what the browser
// believes, because a token's contents are visible but its signature is not
// forgeable.
export function currentRole(): 'owner' | 'admin' | 'member' | null {
  const t = token.get()
  if (!t) return null
  try {
    const payload = JSON.parse(atob(t.split('.')[1]))
    return payload.role ?? null
  } catch {
    return null
  }
}

class ApiError extends Error {
  // Declared explicitly rather than as a constructor parameter property:
  // parameter properties emit runtime code, which erasableSyntaxOnly forbids.
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  const t = token.get()
  if (t) headers.set('Authorization', `Bearer ${t}`)

  const res = await fetch(`/api/v1${path}`, { ...init, headers })

  // A 401 means two different things depending on where it came from. On the
  // auth endpoints it means the credentials were wrong and there is no session
  // to clear; anywhere else it means the token has expired and the user needs
  // to sign in again. Treating them the same shows "session expired" to someone
  // who simply mistyped a password.
  if (res.status === 401) {
    const isAuthAttempt = path.startsWith('/auth/')
    if (!isAuthAttempt) {
      token.clear()
      throw new ApiError(401, 'Your session expired. Sign in again.')
    }
    const body = await res.json().catch(() => ({ error: 'Invalid email or password.' }))
    throw new ApiError(401, body.error ?? 'Invalid email or password.')
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, body.error ?? 'request failed')
  }
  return res.status === 204 ? (undefined as T) : res.json()
}

export const api = {
  signup: (company: string, email: string, password: string) =>
    request<{ token: string; slug: string }>('/auth/signup', {
      method: 'POST',
      body: JSON.stringify({ company, email, password }),
    }),

  forgotPassword: (email: string) =>
    request<{ status: string }>('/auth/forgot', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),

  login: (email: string, password: string) =>
    request<{ token: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  listMonitors: () => request<Monitor[]>('/monitors'),

  listIncidents: () => request<IncidentRow[]>('/incidents'),

  createMonitor: (m: {
    name: string
    type: string
    target: string
    interval_seconds: number
    keyword?: string
    keyword_present?: boolean
    alert_delay_seconds?: number
  }) => request<{ id: string }>('/monitors', { method: 'POST', body: JSON.stringify(m) }),

  deleteMonitor: (id: string) =>
    request<void>(`/monitors/${id}`, { method: 'DELETE' }),

  results: (id: string) => request<CheckResult[]>(`/monitors/${id}/results`),
  updateMonitor: (
    id: string,
    patch: Partial<{
      name: string
      target: string
      interval_seconds: number
      enabled: boolean
      alert_delay_seconds: number
      public: boolean
      public_name: string
      keyword: string
      keyword_present: boolean
    }>,
  ) => request<void>(`/monitors/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),

  listChannels: () => request<Channel[]>('/channels'),

  createChannel: (c: {
    name: string
    type: string
    address?: string
    webhook_url?: string
  }) =>
    request<{ id: string; verification_sent: boolean }>('/channels', {
      method: 'POST',
      body: JSON.stringify(c),
    }),

  deleteChannel: (id: string) => request<void>(`/channels/${id}`, { method: 'DELETE' }),

  testChannel: (id: string) =>
    request<{ status: string }>(`/channels/${id}/test`, { method: 'POST' }),

  listMembers: () => request<Member[]>('/team/members'),

  listInvitations: () => request<Invitation[]>('/team/invitations'),

  invite: (email: string, role: string) =>
    request<{ status: string }>('/team/invitations', {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),

  revokeInvitation: (id: string) =>
    request<void>(`/team/invitations/${id}`, { method: 'DELETE' }),

  changeRole: (id: string, role: string) =>
    request<{ status: string }>(`/team/members/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ role }),
    }),

  removeMember: (id: string) => request<void>(`/team/members/${id}`, { method: 'DELETE' }),

  invitationInfo: (token: string) =>
    request<{ tenant: string; email: string; role: string }>(
      `/invitations/info?token=${encodeURIComponent(token)}`,
    ),

  acceptInvitation: (token: string, password: string) =>
    request<{ token: string }>('/invitations/accept', {
      method: 'POST',
      body: JSON.stringify({ token, password }),
    }),

  currentPlan: () => request<PlanUsage>('/billing/plan'),

  listPlans: () => request<Plan[]>('/plans'),

  account: () => request<Account>('/account'),

  changePassword: (current_password: string, new_password: string) =>
    request<{ status: string }>('/account/password', {
      method: 'POST',
      body: JSON.stringify({ current_password, new_password }),
    }),

  deleteAccount: (confirm: string) =>
    request<{ status: string }>('/account', {
      method: 'DELETE',
      body: JSON.stringify({ confirm }),
    }),

  checkout: (planCode: string) =>
    request<{ url: string }>('/billing/checkout', {
      method: 'POST',
      body: JSON.stringify({ plan_code: planCode }),
    }),

  billingPortal: () =>
    request<{ url: string }>('/billing/portal', { method: 'POST', body: '{}' }),

  listMaintenance: () => request<MaintenanceWindow[]>('/maintenance'),

  createMaintenance: (w: {
    title: string
    description?: string
    starts_at: string
    ends_at: string
    monitor_ids: string[]
  }) => request<{ id: string }>('/maintenance', { method: 'POST', body: JSON.stringify(w) }),

  deleteMaintenance: (id: string) =>
    request<void>(`/maintenance/${id}`, { method: 'DELETE' }),

  monitorChannels: (id: string) =>
    request<{ channel_ids: string[] }>(`/monitors/${id}/channels`),

  monitorDetail: (id: string, window: '24h' | '7d' | '30d') =>
    request<MonitorDetail>(`/monitors/${id}/detail?window=${window}`),

  setMonitorChannels: (id: string, channelIds: string[]) =>
    request<void>(`/monitors/${id}/channels`, {
      method: 'PUT',
      body: JSON.stringify({ channel_ids: channelIds }),
    }),

  statusPageSettings: () => request<StatusPageSettings>('/status-page'),

  updateStatusPage: (p: {
    title?: string
    description?: string
    support_url?: string
    hide_branding?: boolean
  }) => request<void>('/status-page', { method: 'PATCH', body: JSON.stringify(p) }),







}

export { ApiError }

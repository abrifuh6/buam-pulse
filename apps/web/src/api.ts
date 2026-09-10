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

  if (res.status === 401) {
    token.clear()
    throw new ApiError(401, 'session expired')
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

  login: (email: string, password: string) =>
    request<{ token: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  listMonitors: () => request<Monitor[]>('/monitors'),

  createMonitor: (m: {
    name: string
    type: string
    target: string
    interval_seconds: number
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


}

export { ApiError }

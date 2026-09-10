import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type Member, type Invitation } from './api'

export default function Team() {
  const role = currentRole()
  const canInvite = role === 'owner' || role === 'admin'
  const isOwner = role === 'owner'

  const [members, setMembers] = useState<Member[]>([])
  const [invites, setInvites] = useState<Invitation[]>([])
  const [email, setEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('member')
  const [err, setErr] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const [m, i] = await Promise.all([api.listMembers(), api.listInvitations()])
      setMembers(m)
      setInvites(i)
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function invite() {
    setErr('')
    setNote('')
    setBusy(true)
    try {
      await api.invite(email, inviteRole)
      setEmail('')
      setNote('Invitation sent.')
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  async function changeRole(id: string, newRole: string, label: string) {
    if (
      newRole === 'owner' &&
      !confirm(`Transfer ownership to ${label}? You will become an admin.`)
    )
      return
    setErr('')
    try {
      await api.changeRole(id, newRole)
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'failed')
    }
  }

  async function remove(id: string, label: string) {
    if (!confirm(`Remove ${label} from the team?`)) return
    await api.removeMember(id)
    load()
  }

  return (
    <>
      {canInvite && (
        <div className="card">
          <div className="form-grid">
            <input
              placeholder="teammate@company.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
            <select value={inviteRole} onChange={(e) => setInviteRole(e.target.value)}>
              <option value="member">Member</option>
              <option value="admin">Admin</option>
            </select>
            <span className="metric">
              {inviteRole === 'admin'
                ? 'Can manage monitors, alerts and invites'
                : 'Read-only access'}
            </span>
            <span />
            <button onClick={invite} disabled={busy || !email}>
              Invite
            </button>
          </div>
          {err && <p className="err">{err}</p>}
          {note && (
            <p className="muted" style={{ marginTop: 10, fontSize: 13 }}>
              {note}
            </p>
          )}
        </div>
      )}

      {members.map((m) => (
        <div className="card" key={m.id}>
          <div className="row">
            <span className={`dot ${m.verified ? 'up' : 'unknown'}`} />
            <div className="grow">
              <div className="name">
                {m.email}
                {m.is_you && <span className="metric"> · you</span>}
              </div>
              <div className="target">
                {m.role}
                {!m.verified && ' · email not confirmed'}
              </div>
            </div>
            {isOwner && !m.is_you && m.role !== 'owner' && (
              <>
                <select
                  value={m.role}
                  onChange={(e) => changeRole(m.id, e.target.value, m.email)}
                >
                  <option value="member">Member</option>
                  <option value="admin">Admin</option>
                  <option value="owner">Make owner</option>
                </select>
                <button className="ghost" onClick={() => remove(m.id, m.email)}>
                  Remove
                </button>
              </>
            )}
          </div>
        </div>
      ))}

      {invites.length > 0 && (
        <>
          <p className="muted" style={{ marginTop: 24, fontSize: 13 }}>
            Pending invitations
          </p>
          {invites.map((i) => (
            <div className="card" key={i.id}>
              <div className="row">
                <span className="dot unknown" />
                <div className="grow">
                  <div className="name">{i.email}</div>
                  <div className="target">
                    invited as {i.role} · expires{' '}
                    {new Date(i.expires_at).toLocaleDateString()}
                  </div>
                </div>
                {canInvite && (
                  <button
                    className="ghost"
                    onClick={async () => {
                      await api.revokeInvitation(i.id)
                      load()
                    }}
                  >
                    Revoke
                  </button>
                )}
              </div>
            </div>
          ))}
        </>
      )}
    </>
  )
}

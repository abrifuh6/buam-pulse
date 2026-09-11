import { useCallback, useEffect, useState } from 'react'
import { api, currentRole, type Invitation, type Member } from './api'

const ROLE_HELP: Record<string, string> = {
  owner: 'Everything, including billing and deleting the account.',
  admin: 'Manage monitors, alerts and invitations. No billing.',
  member: 'Read-only.',
}

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
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    try {
      const [m, i] = await Promise.all([api.listMembers(), api.listInvitations()])
      setMembers(m)
      setInvites(i)
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not load the team.')
    } finally {
      setLoading(false)
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
      setNote('Invitation sent. It expires in seven days.')
      load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Could not send the invitation.')
    } finally {
      setBusy(false)
    }
  }

  if (loading) return <div className="skeleton" style={{ height: 220 }} />

  return (
    <>
      {canInvite && (
        <div className="panel">
          <div className="panel-head">
            <h2 className="panel-title">Invite someone</h2>
            <p className="panel-note">{ROLE_HELP[inviteRole]}</p>
          </div>
          <div className="row">
            <div className="field grow">
              <label htmlFor="tm-email">Email</label>
              <input
                id="tm-email"
                placeholder="teammate@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="field" style={{ width: 130 }}>
              <label htmlFor="tm-role">Role</label>
              <select
                id="tm-role"
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value)}
              >
                <option value="member">Member</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <button onClick={invite} disabled={busy || !email} style={{ alignSelf: 'flex-end' }}>
              Send invite
            </button>
          </div>
          {err && <p className="err">{err}</p>}
          {note && <p className="ok-note">{note}</p>}
        </div>
      )}

      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr>
              <th style={{ width: 28 }} />
              <th>Member</th>
              <th>Role</th>
              <th className="right" style={{ width: 200 }} />
            </tr>
          </thead>
          <tbody>
            {members.map((m) => (
              <tr key={m.id}>
                <td>
                  <span
                    className={`state ${m.verified ? 'up' : 'warn'}`}
                    title={m.verified ? 'email confirmed' : 'email not confirmed'}
                  />
                </td>
                <td>
                  <div className="cell-name">
                    {m.email}
                    {m.is_you && <span className="metric"> (you)</span>}
                  </div>
                  <div className="cell-target">{ROLE_HELP[m.role]}</div>
                </td>
                <td>
                  <span className={`badge ${m.role === 'owner' ? 'up' : 'idle'}`}>{m.role}</span>
                </td>
                <td className="right">
                  {isOwner && !m.is_you && m.role !== 'owner' && (
                    <div className="row" style={{ justifyContent: 'flex-end' }}>
                      <select
                        value={m.role}
                        style={{ width: 130 }}
                        onChange={async (e) => {
                          const next = e.target.value
                          if (
                            next === 'owner' &&
                            !confirm(`Transfer ownership to ${m.email}? You become an admin.`)
                          )
                            return
                          await api.changeRole(m.id, next)
                          load()
                        }}
                      >
                        <option value="member">Member</option>
                        <option value="admin">Admin</option>
                        <option value="owner">Make owner</option>
                      </select>
                      <button
                        className="quiet"
                        style={{ color: 'var(--down)' }}
                        onClick={async () => {
                          if (!confirm(`Remove ${m.email} from the team?`)) return
                          await api.removeMember(m.id)
                          load()
                        }}
                      >
                        Remove
                      </button>
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {invites.length > 0 && (
        <>
          <p className="section-label">Waiting to accept</p>
          <div className="table-wrap">
            <table className="data">
              <tbody>
                {invites.map((i) => (
                  <tr key={i.id}>
                    <td style={{ width: 28 }}>
                      <span className="state unknown" />
                    </td>
                    <td>
                      <div className="cell-name">{i.email}</div>
                      <div className="cell-target">
                        invited as {i.role}, expires{' '}
                        {new Date(i.expires_at).toLocaleDateString()}
                      </div>
                    </td>
                    <td className="right">
                      {canInvite && (
                        <button
                          className="quiet"
                          onClick={async () => {
                            await api.revokeInvitation(i.id)
                            load()
                          }}
                        >
                          Revoke
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </>
  )
}

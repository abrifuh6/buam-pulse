import { useState } from 'react'
import { api, currentRole, token, type Account } from './api'

export default function AccountSettings({
  account,
  onLogout,
}: {
  account: Account | null
  onLogout: () => void
}) {
  const isOwner = currentRole() === 'owner'

  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [pwErr, setPwErr] = useState('')
  const [pwNote, setPwNote] = useState('')
  const [pwBusy, setPwBusy] = useState(false)

  const [confirmName, setConfirmName] = useState('')
  const [delErr, setDelErr] = useState('')
  const [delBusy, setDelBusy] = useState(false)
  const [exporting, setExporting] = useState(false)

  async function changePassword() {
    setPwErr('')
    setPwNote('')
    setPwBusy(true)
    try {
      await api.changePassword(current, next)
      setCurrent('')
      setNext('')
      setPwNote('Password updated.')
    } catch (e) {
      setPwErr(e instanceof Error ? e.message : 'Could not change your password.')
    } finally {
      setPwBusy(false)
    }
  }

  // The export is a streamed download rather than an API call whose result we
  // hold in memory: a large account's export can be tens of megabytes, and the
  // browser is better at saving a file than React is at buffering one.
  async function exportData() {
    setExporting(true)
    try {
      const res = await fetch('/api/v1/account/export', {
        headers: { Authorization: `Bearer ${token.get()}` },
      })
      if (!res.ok) throw new Error('export failed')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `pulse-export-${account?.tenant.slug ?? 'account'}.json`
      a.click()
      URL.revokeObjectURL(url)
    } finally {
      setExporting(false)
    }
  }

  async function deleteAccount() {
    setDelErr('')
    setDelBusy(true)
    try {
      await api.deleteAccount(confirmName)
      token.clear()
      onLogout()
    } catch (e) {
      setDelErr(e instanceof Error ? e.message : 'Could not delete the account.')
      setDelBusy(false)
    }
  }

  return (
    <>
      <div className="panel">
        <div className="panel-head">
          <h2 className="panel-title">Password</h2>
          <p className="panel-note">
            Your current password is required, so an unattended session cannot lock you out of
            your own account.
          </p>
        </div>
        <div className="stack" style={{ maxWidth: 380 }}>
          <div className="field">
            <label htmlFor="pw-current">Current password</label>
            <input
              id="pw-current"
              type="password"
              autoComplete="current-password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="pw-new">New password</label>
            <input
              id="pw-new"
              type="password"
              autoComplete="new-password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
            />
            <span className="hint">At least 8 characters.</span>
          </div>
          <div>
            <button onClick={changePassword} disabled={pwBusy || !current || next.length < 8}>
              {pwBusy ? 'Updating…' : 'Change password'}
            </button>
          </div>
        </div>
        {pwErr && <p className="err">{pwErr}</p>}
        {pwNote && <p className="ok-note">{pwNote}</p>}
      </div>

      <div className="panel">
        <div className="spread">
          <div>
            <h2 className="panel-title">Export your data</h2>
            <p className="panel-note">
              Every monitor, check result, incident, member and channel as one JSON file.
              Credentials and payment details are not included.
            </p>
          </div>
          <button className="ghost" onClick={exportData} disabled={exporting}>
            {exporting ? 'Preparing…' : 'Download'}
          </button>
        </div>
      </div>

      {isOwner && (
        <div className="panel" style={{ borderColor: 'var(--down)' }}>
          <div className="panel-head">
            <h2 className="panel-title" style={{ color: 'var(--down)' }}>
              Delete this account
            </h2>
            <p className="panel-note">
              This removes every monitor, all history, every team member, and cancels your
              subscription. It cannot be undone. Export your data first if you want to keep it.
            </p>
          </div>
          <div className="stack" style={{ maxWidth: 380 }}>
            <div className="field">
              <label htmlFor="del-confirm">
                Type <b>{account?.tenant.name}</b> to confirm
              </label>
              <input
                id="del-confirm"
                value={confirmName}
                onChange={(e) => setConfirmName(e.target.value)}
              />
            </div>
            <div>
              <button
                className="danger"
                onClick={deleteAccount}
                disabled={delBusy || confirmName !== account?.tenant.name}
              >
                {delBusy ? 'Deleting…' : 'Delete account permanently'}
              </button>
            </div>
          </div>
          {delErr && <p className="err">{delErr}</p>}
        </div>
      )}
    </>
  )
}

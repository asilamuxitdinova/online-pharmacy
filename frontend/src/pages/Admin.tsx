import { AnimatePresence, motion } from 'motion/react'
import { useState } from 'react'
import { ErrorNote, Field, Modal, Page } from '../components/ui'
import { ApiError, del, get, patch, post } from '../lib/api'
import { useAuth } from '../lib/auth'
import { useI18n } from '../lib/i18n'
import { useToast } from '../lib/toast'
import type { Paged, Role, StaffUser } from '../lib/types'
import { useAsync } from '../lib/useAsync'
import { Secret } from './Staff'

const roleColor: Record<Role, string> = { customer: 'var(--sky)', staff: 'var(--mint)', admin: 'var(--lilac)' }

export default function Admin() {
  const { t } = useI18n()
  const { me } = useAuth()
  const toast = useToast()
  const [role, setRole] = useState<Role | ''>('')
  const [q, setQ] = useState('')
  const list = useAsync((s) => get<Paged<StaffUser>>('/admin/users', { role, q, page_size: 50 }, { signal: s }), [role, q])
  const [error, setError] = useState<ApiError | null>(null)
  const [deleting, setDeleting] = useState<StaffUser | null>(null)

  const act = async (fn: () => Promise<unknown>, msg: string) => {
    setError(null)
    try {
      await fn()
      toast(msg, 'ok')
      list.reload()
    } catch (e) {
      setError(e as ApiError)
    }
  }

  return (
    <Page>
      <div className="stack" style={{ '--gap': '24px' } as React.CSSProperties}>
        <div className="stack" style={{ '--gap': '6px' } as React.CSSProperties}>
          <h1 style={{ fontSize: 'clamp(36px, 5vw, 60px)' }}>{t('nav.admin')}</h1>
          <p className="muted small">Changes to roles and access apply immediately.</p>
        </div>
        <div className="row">
          <input className="input" style={{ maxWidth: 300 }} placeholder="Search" value={q} onChange={(e) => setQ(e.target.value)} />
          {(['', 'customer', 'staff', 'admin'] as const).map((r) => (
            <button key={r || 'all'} className={`chip ${role === r ? 'on' : ''}`} onClick={() => setRole(r)}>
              {r || 'Everyone'}
            </button>
          ))}
        </div>
        <ErrorNote error={error ?? list.error} />
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 16 }}>
          <AnimatePresence>
            {list.data?.data.map((u) => {
              const self = u.id === me?.id
              return (
                <motion.div key={u.id} layout initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: u.disabled ? 0.6 : 1, scale: 1 }} exit={{ opacity: 0, scale: 0.6, rotate: -6 }} className="card stack" style={{ '--gap': '12px' } as React.CSSProperties}>
                  <div className="row" style={{ flexWrap: 'nowrap' }}>
                    <motion.span animate={{ background: roleColor[u.role] }} style={{ width: 44, height: 44, borderRadius: 22, display: 'grid', placeItems: 'center', fontWeight: 800, color: '#12302a', flexShrink: 0 }}>
                      {u.name[0]}
                    </motion.span>
                    <div className="grow" style={{ minWidth: 0 }}>
                      <b>{u.name}</b> {self && <span className="badge plain">you</span>}
                      <div className="small muted" style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{u.email}</div>
                    </div>
                  </div>
                  <div className="small">
                    TIN <Secret value={u.tin} />
                  </div>
                  <div className="tabs" style={{ width: '100%' }}>
                    {(['customer', 'staff', 'admin'] as const).map((r) => (
                      <button
                        key={r}
                        className={`tab ${u.role === r ? 'on' : ''}`}
                        style={{ flex: 1 }}
                        disabled={self}
                        onClick={() => u.role !== r && act(() => patch(`/admin/users/${u.id}/role`, { role: r }), `${u.name} is now ${r}`)}
                      >
                        {u.role === r && <motion.div layoutId={`role-${u.id}`} className="tab-pill" />}
                        <span>{r}</span>
                      </button>
                    ))}
                  </div>
                  <div className="spread">
                    <label className="check small">
                      <Toggle
                        on={!u.disabled}
                        disabled={self}
                        onChange={(on) => act(() => post(`/admin/users/${u.id}/${on ? 'enable' : 'disable'}`), on ? 'Enabled' : 'Disabled: every call now answers 403')}
                      />
                      {u.disabled ? 'Disabled' : 'Active'}
                    </label>
                    <button className="btn ghost sm" style={{ color: 'var(--bad)' }} disabled={self} onClick={() => setDeleting(u)}>
                      Delete…
                    </button>
                  </div>
                </motion.div>
              )
            })}
          </AnimatePresence>
        </div>
      </div>
      <DeleteDialog
        user={deleting}
        onClose={() => setDeleting(null)}
        onDeleted={() => {
          setDeleting(null)
          toast('Deleted for good', 'ok')
          list.reload()
        }}
      />
    </Page>
  )
}

function Toggle({ on, onChange, disabled }: { on: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      disabled={disabled}
      onClick={() => onChange(!on)}
      style={{ width: 44, height: 26, borderRadius: 13, border: 0, padding: 3, cursor: disabled ? 'not-allowed' : 'pointer', background: on ? 'var(--mint)' : 'var(--line-2)', display: 'flex', justifyContent: on ? 'flex-end' : 'flex-start', opacity: disabled ? 0.5 : 1 }}
    >
      <motion.span layout transition={{ type: 'spring', stiffness: 600, damping: 32 }} style={{ width: 20, height: 20, borderRadius: 10, background: '#fff', boxShadow: '0 1px 3px rgba(0,0,0,.3)' }} />
    </button>
  )
}

/** Deleting cannot be undone, so the email has to be typed back, as the API demands. */
function DeleteDialog({ user, onClose, onDeleted }: { user: StaffUser | null; onClose: () => void; onDeleted: () => void }) {
  const [typed, setTyped] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const match = !!user && typed.toLowerCase() === user.email.toLowerCase()
  const go = async () => {
    if (!user) return
    setError(null)
    try {
      await del(`/admin/users/${user.id}?confirm_email=${encodeURIComponent(typed)}`)
      setTyped('')
      onDeleted()
    } catch (e) {
      setError(e as ApiError)
    }
  }
  return (
    <Modal open={!!user} onClose={() => { setTyped(''); onClose() }}>
      {user && (
        <div className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
          <h3 style={{ color: 'var(--bad)' }}>Delete {user.name} forever?</h3>
          <p className="small">Their orders, payments, prescriptions, cards and keys go too. <b>This cannot be undone.</b></p>
          <Field label={`Type ${user.email} to confirm`} error={error?.fieldError('confirm_email')}>
            <input className="input mono" value={typed} onChange={(e) => setTyped(e.target.value)} autoFocus />
          </Field>
          <ErrorNote error={error && !error.body.fields ? error : null} />
          <div className="row">
            <motion.button className="btn danger" disabled={!match} onClick={go} animate={match ? { scale: [1, 1.05, 1] } : {}}>
              Delete permanently
            </motion.button>
            <button className="btn ghost" onClick={onClose}>Keep</button>
          </div>
        </div>
      )}
    </Modal>
  )
}

import { AnimatePresence, motion } from 'motion/react'
import { useId, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { ApiError } from '../lib/api'
import { useI18n, type TKey } from '../lib/i18n'

/** Shows an API error the way the API meant it: code, message, every field. */
export function ErrorNote({ error, onRetry }: { error?: ApiError | null; onRetry?: () => void }) {
  if (!error) return null
  const b = error.body
  return (
    <motion.div className="error-note" role="alert" title={b.request_id ? `${b.code} · ${b.request_id}` : b.code} initial={{ opacity: 0, y: -6 }} animate={{ opacity: 1, y: 0 }}>
      <div>{b.message}</div>
      {b.retry_after ? <div className="small">Try again in {b.retry_after}s.</div> : null}
      {b.fields && b.fields.length > 0 && (
        <ul>
          {b.fields.map((f) => (
            <li key={f.field + f.code}>
              {fieldLabel(f.field)} {f.message}
            </li>
          ))}
        </ul>
      )}
      {onRetry && (
        <button className="btn sm ghost" style={{ marginTop: 8 }} onClick={onRetry}>
          Try again
        </button>
      )}
    </motion.div>
  )
}

// "items[0].quantity" → "Item 1 quantity": readable, still precise.
function fieldLabel(field: string) {
  const s = field.replace(/^card\./, 'card ').replace(/\[(\d+)\]/g, (_, i) => ` ${Number(i) + 1}`).replace(/[._]/g, ' ').trim()
  return s.charAt(0).toUpperCase() + s.slice(1)
}

const tone: Record<string, string> = {
  paid: 'ok',
  approved: 'ok',
  succeeded: 'ok',
  pending_payment: 'warn',
  pending_review: 'warn',
  running: 'warn',
  partially_refunded: '',
  refunded: '',
  cancelled: 'bad',
  rejected: 'bad',
  failed: 'bad',
}

export function StatusBadge({ status }: { status: string }) {
  const { t } = useI18n()
  const key = `status.${status}` as TKey
  let label: string = status.replace(/_/g, ' ')
  try {
    label = t(key) ?? label
  } catch {
    /* unknown status */
  }
  return <span className={`badge ${tone[status] ?? ''}`}>{label}</span>
}

export function Field({
  label,
  error,
  children,
  hint,
}: {
  label: string
  error?: string
  hint?: ReactNode
  children: ReactNode
}) {
  return (
    <label className={`field ${error ? 'invalid' : ''}`}>
      <span>{label}</span>
      {children}
      {error ? <span className="field-error">{error}</span> : hint ? <span className="tiny muted">{hint}</span> : null}
    </label>
  )
}

export function Tabs<T extends string>({ value, onChange, items }: { value: T; onChange: (v: T) => void; items: { id: T; label: ReactNode }[] }) {
  const id = useId()
  return (
    <div className="tabs" role="tablist">
      {items.map((it) => (
        <button key={it.id} role="tab" aria-selected={value === it.id} className={`tab ${value === it.id ? 'on' : ''}`} onClick={() => onChange(it.id)}>
          {value === it.id && <motion.div layoutId={`tab-${id}`} className="tab-pill" transition={{ type: 'spring', stiffness: 500, damping: 38 }} />}
          <span>{it.label}</span>
        </button>
      ))}
    </div>
  )
}

// Portalled to <body>: pages animate with transforms, and a transformed
// ancestor would make position: fixed relative to the page, not the screen.
export function Modal({ open, onClose, children }: { open: boolean; onClose: () => void; children: ReactNode }) {
  return createPortal(
    <AnimatePresence>
      {open && (
        <>
          <motion.div className="scrim" onClick={onClose} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} />
          <motion.div
            className="modal card"
            role="dialog"
            aria-modal="true"
            initial={{ opacity: 0, scale: 0.92, x: '-50%', y: '-46%' }}
            animate={{ opacity: 1, scale: 1, x: '-50%', y: '-50%' }}
            exit={{ opacity: 0, scale: 0.95, x: '-50%', y: '-48%' }}
            transition={{ type: 'spring', stiffness: 420, damping: 32 }}
          >
            {children}
          </motion.div>
        </>
      )}
    </AnimatePresence>,
    document.body,
  )
}

export function Skeleton({ h = 20, w = '100%', r }: { h?: number; w?: number | string; r?: number }) {
  return <div className="skeleton" style={{ height: h, width: w, borderRadius: r }} />
}

export function Empty({ title, hint, art }: { title: string; hint?: string; art?: ReactNode }) {
  return (
    <motion.div className="card flat center stack" style={{ '--gap': '10px', alignItems: 'center', padding: 40 } as React.CSSProperties} initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
      {art}
      <h3>{title}</h3>
      {hint && <p className="muted small" style={{ maxWidth: 420 }}>{hint}</p>}
    </motion.div>
  )
}

export function Loading() {
  const { t } = useI18n()
  return (
    <div className="row muted small" style={{ padding: 16 }}>
      <motion.span
        style={{ width: 22, height: 10, borderRadius: 5, background: 'linear-gradient(90deg, var(--coral) 50%, var(--mint) 50%)', display: 'inline-block' }}
        animate={{ rotate: 360 }}
        transition={{ repeat: Infinity, duration: 1.1, ease: 'linear' }}
      />
      {t('common.loading')}
    </div>
  )
}

/** Fades and lifts a page in; used as the child of AnimatePresence in the router. */
export function Page({ children, wide }: { children: ReactNode; wide?: boolean }) {
  return (
    <motion.main
      className="page"
      initial={{ opacity: 0, y: 16 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      transition={{ duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
    >
      <div className="wrap" style={wide ? { width: 'min(1440px, 100%)' } : undefined}>
        {children}
      </div>
    </motion.main>
  )
}

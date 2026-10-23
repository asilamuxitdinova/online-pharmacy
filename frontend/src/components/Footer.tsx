import { Link } from 'react-router'
import { useI18n } from '../lib/i18n'

// Policy pages are rendered by the API server under /docs.
const policies = [
  { href: '/docs/delivery', key: 'footer.delivery' },
  { href: '/docs/refunds', key: 'footer.refunds' },
  { href: '/docs/prescriptions', key: 'footer.prescriptions' },
  { href: '/docs/payments', key: 'footer.payments' },
  { href: '/docs/faq', key: 'footer.faq' },
] as const

export function Footer() {
  const { t } = useI18n()
  return (
    <footer style={{ borderTop: '1px solid var(--line)', padding: '40px 0 56px', marginTop: 40 }}>
      <div className="wrap spread" style={{ alignItems: 'flex-start', gap: 32 }}>
        <div className="stack" style={{ '--gap': '8px', maxWidth: 360 } as React.CSSProperties}>
          <Link to="/" className="logo" style={{ fontSize: 20 }}>Dorixona</Link>
          <p className="small muted">{t('footer.tagline')}</p>
        </div>
        <nav className="stack small" style={{ '--gap': '6px' } as React.CSSProperties} aria-label={t('footer.help')}>
          <b>{t('footer.help')}</b>
          {policies.map((p) => (
            <a key={p.href} href={p.href} className="muted" style={{ textDecoration: 'none' }}>
              {t(p.key)}
            </a>
          ))}
        </nav>
        <nav className="stack small" style={{ '--gap': '6px' } as React.CSSProperties} aria-label={t('footer.developers')}>
          <b>{t('footer.developers')}</b>
          <a href="/docs/" className="muted" style={{ textDecoration: 'none' }}>{t('footer.apiDocs')}</a>
          <a href="/openapi.yaml" className="muted" style={{ textDecoration: 'none' }}>OpenAPI</a>
        </nav>
      </div>
      <div className="wrap tiny muted" style={{ marginTop: 28 }}>© 2026 Dorixona</div>
    </footer>
  )
}

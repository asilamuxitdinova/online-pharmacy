import { AnimatePresence, motion } from 'motion/react'
import { useState } from 'react'
import { Link, NavLink, useLocation } from 'react-router'
import { useAuth } from '../lib/auth'
import { useCart } from '../lib/cart'
import { useI18n } from '../lib/i18n'
import type { Lang } from '../lib/types'

function Logo() {
  return (
    <Link to="/" className="logo" aria-label="Dorixona home">
      <motion.svg width="30" height="30" viewBox="0 0 32 32" whileHover={{ rotate: 180 }} transition={{ type: 'spring', stiffness: 200, damping: 12 }}>
        <g transform="rotate(-45 16 16)">
          <rect x="3" y="10.5" width="26" height="11" rx="5.5" fill="var(--coral)" />
          <path d="M16 10.5h7.5a5.5 5.5 0 0 1 0 11H16z" fill="var(--mint)" />
        </g>
      </motion.svg>
      <span>Dorixona</span>
    </Link>
  )
}

export function Nav() {
  const { t, lang, setLang } = useI18n()
  const { me, is, signOut, actingAs, actAs } = useAuth()
  const cart = useCart()
  const [menu, setMenu] = useState(false)
  const loc = useLocation()

  const links = [
    { to: '/shop', label: t('nav.shop'), show: true },
    { to: '/account', label: t('nav.account'), show: !!me },
    { to: '/staff', label: t('nav.staff'), show: is('staff') && !actingAs },
    { to: '/admin', label: t('nav.admin'), show: is('admin') && !actingAs },
  ].filter((l) => l.show)

  const toggleTheme = () => {
    const root = document.documentElement
    const dark = root.dataset.theme ? root.dataset.theme === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches
    root.dataset.theme = dark ? 'light' : 'dark'
    try {
      localStorage.setItem('dorixona.theme', root.dataset.theme)
    } catch {
      /* ignore */
    }
  }

  return (
    <>
      <header className="nav">
        <div className="wrap">
          <div className="nav-inner">
            <Logo />
            <nav className="nav-links">
              {links.map((l) => (
                <NavLink key={l.to} to={l.to} className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
                  {({ isActive }) => (
                    <>
                      {isActive && <motion.span layoutId="nav-pill" className="nav-pill" transition={{ type: 'spring', stiffness: 500, damping: 38 }} />}
                      {l.label}
                    </>
                  )}
                </NavLink>
              ))}
            </nav>
            <div className="grow" />
            <select className="chip" style={{ paddingRight: 8 }} value={lang} onChange={(e) => setLang(e.target.value as Lang)} aria-label="Language">
              <option value="en">EN</option>
              <option value="ru">RU</option>
              <option value="uz">UZ</option>
            </select>
            <button className="btn ghost sm icon hide-sm" onClick={toggleTheme} aria-label="Toggle dark mode" title="Light / dark">
              ◐
            </button>
            {me ? (
              <button className="chip hide-sm" onClick={() => void signOut()} title={me.email}>
                <span style={{ width: 22, height: 22, borderRadius: '50%', background: 'var(--mint)', color: '#08241c', display: 'grid', placeItems: 'center', fontSize: 11, fontWeight: 800 }}>
                  {me.name.slice(0, 1)}
                </span>
                {t('nav.signout')}
              </button>
            ) : (
              <Link to="/login" className="btn sm" state={{ from: loc.pathname }}>
                {t('nav.signin')}
              </Link>
            )}
            <motion.button className="btn coral sm icon" style={{ position: 'relative' }} onClick={() => cart.setOpen(true)} aria-label={t('nav.cart')} whileTap={{ scale: 0.9 }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M5 8h14l-1.2 11.2A2 2 0 0 1 15.8 21H8.2a2 2 0 0 1-2-1.8z" />
                <path d="M9 8V6a3 3 0 0 1 6 0v2" />
              </svg>
              <AnimatePresence>
                {cart.count > 0 && (
                  <motion.span key={cart.count} className="cart-count" initial={{ scale: 0 }} animate={{ scale: 1 }} exit={{ scale: 0 }}>
                    {cart.count}
                  </motion.span>
                )}
              </AnimatePresence>
            </motion.button>
            <button className="btn ghost sm icon burger" onClick={() => setMenu((m) => !m)} aria-label="Menu" aria-expanded={menu}>
              ☰
            </button>
          </div>
        </div>
      </header>
      <AnimatePresence>
        {menu && (
          <motion.nav className="mobile-menu card" initial={{ opacity: 0, y: -10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} onClick={() => setMenu(false)}>
            {links.map((l) => (
              <NavLink key={l.to} to={l.to}>
                {l.label}
              </NavLink>
            ))}
            {me && (
              <a href="#" onClick={() => void signOut()}>
                {t('nav.signout')} ({me.email})
              </a>
            )}
            <a href="#" onClick={toggleTheme}>
              ◐ Light / dark
            </a>
          </motion.nav>
        )}
      </AnimatePresence>
      <AnimatePresence>
        {actingAs && me && (
          <motion.div className="act-banner" initial={{ y: 80 }} animate={{ y: 0 }} exit={{ y: 80 }}>
            <span>
              Viewing as {me.name}
            </span>
            <button className="btn sm" onClick={() => actAs(null)}>
              Stop
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  )
}

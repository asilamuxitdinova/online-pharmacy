import { AnimatePresence, MotionConfig } from 'motion/react'
import { lazy, Suspense, useEffect } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router'
import { CartDrawer } from './components/CartDrawer'
import { Nav } from './components/Nav'
import { Loading } from './components/ui'
import { useAuth } from './lib/auth'
import type { Role } from './lib/types'
import Home from './pages/Home'
import Shop from './pages/Shop'

const Product = lazy(() => import('./pages/Product'))
const Checkout = lazy(() => import('./pages/Checkout'))
const Login = lazy(() => import('./pages/Login'))
const OAuthCallback = lazy(() => import('./pages/OAuthCallback'))
const Account = lazy(() => import('./pages/Account'))
const OrderDetail = lazy(() => import('./pages/OrderDetail'))
const Staff = lazy(() => import('./pages/Staff'))
const Admin = lazy(() => import('./pages/Admin'))
const Lab = lazy(() => import('./pages/Lab'))
const NotFound = lazy(() => import('./pages/NotFound'))

/** Sends visitors who are not signed in (or lack the role) somewhere sensible. */
function Guard({ role, children }: { role: Role; children: React.ReactNode }) {
  const { me, loading, is, creds } = useAuth()
  const loc = useLocation()
  if (loading && creds) return <Loading />
  if (!me) return <Navigate to="/login" replace state={{ from: loc.pathname }} />
  if (!is(role)) return <Navigate to="/account" replace />
  return <>{children}</>
}

export default function App() {
  const loc = useLocation()
  useEffect(() => {
    window.scrollTo({ top: 0 })
  }, [loc.pathname])
  return (
    <MotionConfig reducedMotion="user">
      <Nav />
      <CartDrawer />
      <Suspense fallback={<div className="page wrap"><Loading /></div>}>
        <AnimatePresence mode="wait">
          <Routes location={loc} key={loc.pathname}>
            <Route path="/" element={<Home />} />
            <Route path="/shop" element={<Shop />} />
            <Route path="/p/:id" element={<Product />} />
            <Route path="/checkout" element={<Guard role="customer"><Checkout /></Guard>} />
            <Route path="/login" element={<Login />} />
            <Route path="/oauth/callback" element={<OAuthCallback />} />
            <Route path="/account" element={<Guard role="customer"><Account /></Guard>} />
            <Route path="/orders/:id" element={<Guard role="customer"><OrderDetail /></Guard>} />
            <Route path="/staff" element={<Guard role="staff"><Staff /></Guard>} />
            <Route path="/admin" element={<Guard role="admin"><Admin /></Guard>} />
            <Route path="/lab" element={<Lab />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </AnimatePresence>
      </Suspense>
    </MotionConfig>
  )
}

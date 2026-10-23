import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import type { Product } from './types'

export interface CartLine {
  product: Product
  quantity: number
}

interface Cart {
  lines: CartLine[]
  open: boolean
  setOpen: (v: boolean) => void
  add: (p: Product, qty?: number) => void
  setQty: (id: string, qty: number) => void
  remove: (id: string) => void
  clear: () => void
  count: number
  subtotal: number
  needsPrescription: boolean
}

const Ctx = createContext<Cart | null>(null)
const KEY = 'dorixona.cart'

export function CartProvider({ children }: { children: ReactNode }) {
  const [lines, setLines] = useState<CartLine[]>(() => {
    try {
      return JSON.parse(localStorage.getItem(KEY) ?? '[]') as CartLine[]
    } catch {
      return []
    }
  })
  const [open, setOpen] = useState(false)
  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify(lines))
    } catch {
      /* ignore */
    }
  }, [lines])

  const setQty = (id: string, qty: number) =>
    setLines((ls) => ls.map((l) => (l.product.id === id ? { ...l, quantity: Math.max(1, Math.min(10, qty)) } : l)))
  const value: Cart = {
    lines,
    open,
    setOpen,
    add: (p, qty = 1) =>
      setLines((ls) =>
        ls.some((l) => l.product.id === p.id)
          ? ls.map((l) => (l.product.id === p.id ? { ...l, quantity: Math.min(10, l.quantity + qty) } : l))
          : [...ls, { product: p, quantity: qty }],
      ),
    setQty,
    remove: (id) => setLines((ls) => ls.filter((l) => l.product.id !== id)),
    clear: () => setLines([]),
    count: lines.reduce((n, l) => n + l.quantity, 0),
    subtotal: lines.reduce((n, l) => n + l.quantity * l.product.price, 0),
    needsPrescription: lines.some((l) => l.product.requires_prescription),
  }
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useCart() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useCart outside CartProvider')
  return v
}

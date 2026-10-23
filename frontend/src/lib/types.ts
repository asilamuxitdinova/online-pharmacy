// Shapes the pharmacy API answers with. They mirror openapi.yaml.

export type Role = 'customer' | 'staff' | 'admin'
export type Lang = 'en' | 'ru' | 'uz'

export interface User {
  id: string
  email: string
  name: string
  phone: string | null
  role: Role
  created_at: string
}

export interface Me extends User {
  auth_method: 'bearer' | 'cookie' | 'api_key'
  session_id?: string
  acting_as?: boolean
  actor?: { id: string; name: string; role: Role }
}

export interface StaffUser extends User {
  tin: string | null
  disabled: boolean
  disabled_at: string | null
  order_count?: number
  prescription_count?: number
  lifetime_spent?: number
}

export interface TokenSet {
  access_token: string
  token_type: 'Bearer'
  expires_in: number
  refresh_token: string
  refresh_expires_in: number
  session_id: string
  user: User
}

export type ProductForm = 'tablets' | 'capsules' | 'sachets' | 'syrup' | 'drops' | 'cream' | 'spray'

export interface Product {
  id: string
  sku: string
  name: string
  category: { id: string; name: string }
  manufacturer: string
  form: ProductForm
  strength: string
  pack_size: number
  price: number
  currency: 'UZS'
  stock: number
  in_stock: boolean
  requires_prescription: boolean
}

export interface Category {
  id: string
  name: string
  product_count: number
}

export interface Paged<T> {
  data: T[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export interface Cursored<T> {
  data: T[]
  next_cursor: string | null
  has_more: boolean
}

export type OrderStatus = 'pending_payment' | 'paid' | 'cancelled' | 'partially_refunded' | 'refunded'

export interface OrderItem {
  product_id: string
  name: string
  quantity: number
  unit_price: number
  line_total: number
}

export interface Order {
  id: string
  user_id: string
  status: OrderStatus
  items: OrderItem[]
  delivery: 'courier' | 'pickup'
  address_id: string | null
  prescription_id: string | null
  note: string
  subtotal: number
  delivery_fee: number
  total: number
  refunded: number
  currency: 'UZS'
  created_at: string
  updated_at: string
  paid_at: string | null
}

export interface Address {
  id: string
  label: string
  line1: string
  city: string
  phone: string
  created_at: string
}

export interface Card {
  id: string
  brand: 'visa' | 'mastercard' | 'uzcard' | 'humo'
  last4: string
  exp_month: number
  exp_year: number
  created_at: string
}

export interface ApiKey {
  id: string
  name: string
  prefix: string
  created_at: string
  revoked_at: string | null
  key?: string
}

export interface Prescription {
  id: string
  user_id: string
  doctor_name: string
  clinic: string
  issued_on: string
  diagnosis: string
  status: 'pending_review' | 'approved' | 'rejected'
  review_note: string | null
  reviewed_by: string | null
  created_at: string
}

export interface Refund {
  id: string
  order_id: string
  amount: number
  currency: 'UZS'
  reason: string
  status: 'approved' | 'pending_review' | 'rejected'
  decided_by: string | null
  created_at: string
  decided_at: string | null
  auto_approved?: boolean
  rule?: string
}

export interface Job {
  id: string
  kind: 'stock_check' | 'sales_report'
  status: 'running' | 'succeeded' | 'failed'
  progress: number
  estimated_seconds_left?: number
  input: Record<string, string>
  result?: Record<string, unknown>
  error?: { code: string; message: string }
  created_at: string
  finished_at?: string
}

export interface StockResult {
  product_id: string
  name: string
  online_stock: number
  branch_total: number
  branches: { branch: string; city: string; available: number }[]
}

export interface InboxMessage {
  id: number
  kind: string
  recipient: string
  subject: string
  body: string
  meta: Record<string, unknown>
  created_at: string
}

export interface FieldError {
  field: string
  code: string
  message: string
}

export interface ApiErrorBody {
  status: number
  code: string
  message: string
  required_role?: Role
  retry_after?: number
  fields?: FieldError[]
  details?: Record<string, unknown>
  request_id?: string
}

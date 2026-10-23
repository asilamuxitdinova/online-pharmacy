import type { Lang } from './types'

const locales: Record<Lang, string> = { en: 'en-GB', ru: 'ru-RU', uz: 'uz-Latn-UZ' }

export function money(n: number, lang: Lang = 'en') {
  const s = new Intl.NumberFormat(locales[lang]).format(n).replace(/[  ,]/g, ' ')
  return `${s} ${lang === 'ru' ? 'сум' : lang === 'uz' ? "so'm" : 'UZS'}`
}

export function when(iso: string | null | undefined, lang: Lang = 'en') {
  if (!iso) return '—'
  return new Intl.DateTimeFormat(locales[lang], { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

export function day(iso: string, lang: Lang = 'en') {
  return new Intl.DateTimeFormat(locales[lang], { dateStyle: 'medium' }).format(new Date(iso))
}

/** A stable hue from a string: every product gets its own pill colours. */
export function hue(seed: string) {
  let h = 0
  for (const c of seed) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return h % 360
}

export function pillColors(seed: string): [string, string] {
  const h = hue(seed)
  // Comma syntax: three.js does not parse the space-separated form.
  return [`hsl(${h}, 82%, 62%)`, `hsl(${(h + 140) % 360}, 70%, 88%)`]
}

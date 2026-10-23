import type { ProductForm } from '../lib/types'

/** A flat drawing of a product's form, used where a 3D canvas would be too heavy. */
export function PillIcon({ form, a, b, size = 96 }: { form: ProductForm; a: string; b: string; size?: number }) {
  const common = { width: size, height: size, viewBox: '0 0 100 100', 'aria-hidden': true }
  const shine = <ellipse cx="38" cy="34" rx="10" ry="4" fill="#fff" opacity=".45" transform="rotate(-35 38 34)" />
  switch (form) {
    case 'capsules':
      return (
        <svg {...common}>
          <g transform="rotate(-38 50 50)">
            <rect x="14" y="36" width="72" height="28" rx="14" fill={b} />
            <path d="M50 36h22a14 14 0 0 1 0 28H50z" fill={a} />
            <rect x="20" y="40" width="22" height="5" rx="2.5" fill="#fff" opacity=".5" />
          </g>
        </svg>
      )
    case 'tablets':
      return (
        <svg {...common}>
          <ellipse cx="50" cy="58" rx="34" ry="14" fill={a} filter="brightness(.8)" />
          <ellipse cx="50" cy="52" rx="34" ry="14" fill={a} />
          <path d="M24 52h52" stroke="#000" strokeOpacity=".15" strokeWidth="2.5" strokeLinecap="round" />
          {shine}
        </svg>
      )
    case 'syrup':
    case 'drops': {
      const s = form === 'drops' ? 0.8 : 1
      return (
        <svg {...common}>
          <g transform={`translate(${50 - 50 * s} ${54 - 50 * s}) scale(${s})`}>
            <rect x="40" y="10" width="20" height="12" rx="3" fill={a} />
            <path d="M42 22h16v8c10 4 14 10 14 18v36a6 6 0 0 1-6 6H34a6 6 0 0 1-6-6V48c0-8 4-14 14-18z" fill="#9a5a2a" />
            <rect x="28" y="52" width="44" height="22" fill={b} />
            <rect x="28" y="60" width="44" height="4" fill={a} />
            <rect x="33" y="34" width="5" height="44" rx="2.5" fill="#fff" opacity=".25" />
          </g>
        </svg>
      )
    }
    case 'cream':
      return (
        <svg {...common}>
          <g transform="rotate(-30 50 50)">
            <path d="M30 22h40l-6 52H36z" fill={b} />
            <rect x="28" y="16" width="44" height="8" rx="2" fill={b} filter="brightness(.9)" />
            <rect x="42" y="74" width="16" height="12" rx="3" fill={a} />
            <rect x="33" y="40" width="34" height="10" fill={a} />
          </g>
        </svg>
      )
    case 'spray':
      return (
        <svg {...common}>
          <rect x="44" y="8" width="12" height="12" rx="3" fill={a} />
          <rect x="56" y="11" width="10" height="4" rx="2" fill="#333" />
          <path d="M34 30a16 12 0 0 1 32 0v56a4 4 0 0 1-4 4H38a4 4 0 0 1-4-4z" fill={b} />
          <rect x="34" y="44" width="32" height="20" fill={a} />
          <rect x="38" y="32" width="5" height="52" rx="2.5" fill="#fff" opacity=".3" />
        </svg>
      )
    case 'sachets':
      return (
        <svg {...common}>
          <g transform="rotate(8 50 50)">
            <rect x="20" y="20" width="60" height="60" rx="6" fill={b} />
            <path d="M20 28h60M20 72h60" stroke="#000" strokeOpacity=".12" strokeDasharray="3 3" strokeWidth="2" />
            <circle cx="50" cy="50" r="13" fill={a} />
          </g>
        </svg>
      )
  }
}

import { useId, type ReactNode } from 'react'

/**
 * Verification seals. One shape, four meanings — the colour and glyph tell them apart:
 *
 *   telegram  sky → blue      account is linked to a real Telegram account
 *   phone     mint → emerald  phone number confirmed
 *   photo     lilac → violet  live selfie matched the profile photos
 *   identity  gold → amber    government ID checked (highest trust)
 *   plus      rose → violet   Atish Plus member
 *
 * The palette is tuned to sit next to the brand gradient (#FF3B5C → #8B5CF6)
 * and keeps enough contrast for the white glyph on both light and dark themes.
 */
export type VerifyKind = 'telegram' | 'phone' | 'photo' | 'identity' | 'plus'

export const VERIFY_META: Record<VerifyKind, { label: string; hint: string; from: string; to: string; tint: string }> = {
  telegram: { label: 'Telegram', hint: 'Linked to a real Telegram account', from: '#38BDF8', to: '#2563EB', tint: '#2563EB' },
  phone: { label: 'Phone', hint: 'Phone number confirmed', from: '#4ADE80', to: '#059669', tint: '#059669' },
  photo: { label: 'Photo', hint: 'Live selfie matched their photos', from: '#C4B5FD', to: '#7C3AED', tint: '#7C3AED' },
  identity: { label: 'ID', hint: 'Government ID checked', from: '#FDE047', to: '#D97706', tint: '#B45309' },
  plus: { label: 'Plus', hint: 'Atish Plus member', from: '#FF3B5C', to: '#8B5CF6', tint: '#D13FA8' },
}

// A 24×24 circle with a gently scalloped edge (10 soft bumps), built once.
const SEAL = (() => {
  const pts: string[] = []
  const N = 120
  for (let i = 0; i < N; i++) {
    const t = (i / N) * Math.PI * 2
    const r = 10.6 + 1.05 * Math.cos(10 * t)
    pts.push(`${(12 + r * Math.cos(t)).toFixed(2)} ${(12 + r * Math.sin(t)).toFixed(2)}`)
  }
  return `M${pts.join('L')}Z`
})()

const G = { fill: 'none', stroke: '#fff', strokeWidth: 1.9, strokeLinecap: 'round', strokeLinejoin: 'round' } as const

const GLYPH: Record<VerifyKind, ReactNode> = {
  telegram: <path d="M17.6 7.1 6.6 11.3l3.2 1.2 1.2 3.9 1.9-1.9 3.1 2.3 2.2-9.700Z" fill="#fff" />,
  phone: (
    <g {...G}>
      <rect x="8.6" y="6" width="6.8" height="12" rx="1.9" />
      <path d="M11 15.6h2" />
    </g>
  ),
  photo: (
    <g {...G}>
      <path d="M8.600 8.800h1.100l.9-1.400h2.800l.9 1.400h1.100c1 0 1.700.8 1.700 1.700v4.800c0 1-.8 1.700-1.700 1.700H8.600c-1 0-1.700-.7-1.700-1.700v-4.800c0-.9.7-1.700 1.700-1.700Z" />
      <circle cx="12" cy="13" r="2.100" />
    </g>
  ),
  identity: (
    <g {...G}>
      <path d="M12 6.400 7.600 8.100v3.600c0 2.700 1.800 4.600 4.400 5.800 2.600-1.200 4.400-3.100 4.400-5.800V8.100Z" />
      <path d="m9.900 12 1.500 1.600 2.800-3.100" />
    </g>
  ),
  plus: <path d="M12 6.200c.4 2.600 1.200 3.400 3.800 3.800-2.600.4-3.400 1.200-3.800 3.800-.4-2.600-1.200-3.400-3.800-3.800 2.600-.4 3.400-1.200 3.800-3.800Zm4.400 7.200c.2 1.200.6 1.600 1.800 1.800-1.200.2-1.600.6-1.800 1.800-.2-1.200-.6-1.600-1.800-1.800 1.200-.2 1.600-.6 1.800-1.800Z" fill="#fff" />,
}

/** The seal on its own — use next to names. */
export function VerifyBadge({ kind, size = 18, className = '' }: { kind: VerifyKind; size?: number; className?: string }) {
  const id = useId().replace(/:/g, '')
  const m = VERIFY_META[kind]
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" role="img" aria-label={`${m.label} verified`} className={`inline-block shrink-0 align-[-0.2em] ${className}`}>
      <title>{m.hint}</title>
      <defs>
        <linearGradient id={`g${id}`} x1="0.1" y1="0" x2="0.9" y2="1">
          <stop offset="0" stopColor={m.from} />
          <stop offset="1" stopColor={m.to} />
        </linearGradient>
        <linearGradient id={`s${id}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#fff" stopOpacity=".38" />
          <stop offset=".55" stopColor="#fff" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={SEAL} fill={`url(#g${id})`} />
      <path d={SEAL} fill={`url(#s${id})`} />
      <g>{GLYPH[kind]}</g>
    </svg>
  )
}

/** Seal + label on a tinted pill — use where the meaning should be readable. */
export function VerifyPill({ kind, label, muted = false, onClick }: { kind: VerifyKind; label?: string; muted?: boolean; onClick?: () => void }) {
  const m = VERIFY_META[kind]
  const Tag = onClick ? 'button' : 'span'
  return (
    <Tag
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 rounded-full py-1 pl-1 pr-3 text-xs font-bold ${onClick ? 'press cursor-pointer' : ''}`}
      style={muted ? { background: 'rgb(var(--elevated))', color: 'rgb(var(--muted))' } : { background: `${m.tint}1A`, color: m.tint }}
    >
      <span className={muted ? 'opacity-40 grayscale' : ''}>
        <VerifyBadge kind={kind} size={20} />
      </span>
      {label ?? m.label}
    </Tag>
  )
}

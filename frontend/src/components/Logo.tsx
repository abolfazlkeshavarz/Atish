import { useId } from 'react'

/** Atish mark: a flame with a heart carved out of it. `tile` renders it white on the brand gradient (app icon). */
export function Logo({ size = 40, tile = false, glow = false, className = '' }: { size?: number; tile?: boolean; glow?: boolean; className?: string }) {
  const id = useId().replace(/:/g, '')
  const flame =
    'M32 5c2.5 10 14 15 14 29a14 14 0 0 1-28 0c0-7 3.5-11 7-14 .2 4.6 2 7 4.500 8.200C28 20 28 11 32 5z'
  // heart cut-out (even-odd) in the lower body of the flame
  const heart = 'M32 48.500c-6.500-4.700-9-8-9-11 0-2.600 2-4.300 4.300-4.300 1.900 0 3.600 1.100 4.700 3 1.100-1.900 2.800-3 4.700-3 2.300 0 4.300 1.700 4.300 4.300 0 3-2.500 6.300-9 11z'
  const style = glow ? { filter: 'drop-shadow(0 8px 18px rgb(var(--brand) / .45))' } : undefined
  if (tile) {
    return (
      <svg width={size} height={size} viewBox="0 0 64 64" className={className} style={style} aria-label="Atish">
        <defs>
          <linearGradient id={`t${id}`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#FF3B5C" />
            <stop offset="1" stopColor="#8B5CF6" />
          </linearGradient>
        </defs>
        <rect width="64" height="64" rx="18" fill={`url(#t${id})`} />
        <g transform="translate(9.600 9.600) scale(.7)">
          <path d={`${flame} ${heart}`} fill="#fff" fillRule="evenodd" />
        </g>
      </svg>
    )
  }
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" className={className} style={style} aria-label="Atish">
      <defs>
        <linearGradient id={`f${id}`} x1=".15" y1="0" x2=".85" y2="1">
          <stop offset="0" stopColor="#FF8A3D" />
          <stop offset=".45" stopColor="#FF3B5C" />
          <stop offset="1" stopColor="#8B5CF6" />
        </linearGradient>
      </defs>
      <path d={`${flame} ${heart}`} fill={`url(#f${id})`} fillRule="evenodd" />
    </svg>
  )
}

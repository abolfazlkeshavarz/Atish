/** Atish mark: the three-tongued flame cradling a heart, on its dark tile (the brand app icon). */
export function Logo({ size = 40, glow = false, className = '' }: { size?: number; tile?: boolean; glow?: boolean; className?: string }) {
  const style = glow ? { filter: 'drop-shadow(0 8px 22px rgb(255 90 60 / .45))' } : undefined
  return <img src="/logo.webp" width={size} height={size} alt="Atish" draggable={false} className={`select-none ${className}`} style={style} />
}

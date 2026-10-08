import { useEffect, useRef, useState, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { X, Check, Loader2 } from 'lucide-react'
import { useUI } from '@/store/ui'
import { platform } from '@/platform/telegram'

export const cx = (...a: (string | false | null | undefined)[]) => a.filter(Boolean).join(' ')

/* ───────── Button ───────── */
type BtnProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'soft' | 'ghost' | 'danger' | 'outline' | 'friend'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  block?: boolean
}

export function Button({ variant = 'primary', size = 'md', loading, block, className, children, disabled, onClick, ...rest }: BtnProps) {
  const v = {
    primary: 'brand-gradient text-white shadow-glow',
    friend: 'friend-gradient text-white',
    soft: 'bg-elevated text-ink',
    ghost: 'bg-transparent text-muted',
    danger: 'bg-danger text-white',
    outline: 'border border-line bg-surface text-ink',
  }[variant]
  const s = { sm: 'h-9 px-4 text-sm', md: 'h-12 px-5 text-[15px]', lg: 'h-14 px-6 text-base' }[size]
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      onClick={(e) => {
        platform.haptic('light')
        onClick?.(e)
      }}
      className={cx(
        'press inline-flex select-none items-center justify-center gap-2 rounded-2xl font-semibold disabled:opacity-45 disabled:active:scale-100',
        v, s, block && 'w-full', className,
      )}
    >
      {loading && <Loader2 className="h-4 w-4 animate-spin" />}
      {children}
    </button>
  )
}

/* ───────── Chip ───────── */
export function Chip({ selected, onClick, children, tone = 'brand', size = 'md', className }: {
  selected?: boolean; onClick?: () => void; children: ReactNode; tone?: 'brand' | 'friend' | 'love' | 'muted'; size?: 'sm' | 'md'; className?: string
}) {
  const on = { brand: 'bg-brand/10 border-brand text-brand', friend: 'bg-friend/10 border-friend text-friend', love: 'bg-love/10 border-love text-love', muted: 'bg-elevated border-line text-ink' }[tone]
  const Tag = onClick ? 'button' : 'span'
  return (
    <Tag
      onClick={onClick ? () => { platform.haptic('select'); onClick() } : undefined}
      className={cx(
        'inline-flex items-center gap-1.5 rounded-full border font-medium transition',
        size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3.5 py-2 text-sm',
        selected ? on : 'border-line bg-surface text-ink',
        onClick && 'press cursor-pointer',
        className,
      )}
    >
      {children}
    </Tag>
  )
}

export function ChipGroup<T extends string>({ options, value, onChange, multi, tone }: {
  options: { v: T; label: string; emoji?: string }[]
  value: T | T[]
  onChange: (v: T) => void
  multi?: boolean
  tone?: 'brand' | 'friend' | 'love'
}) {
  const sel = (v: T) => (multi ? (value as T[]).includes(v) : value === v)
  return (
    <div className="flex flex-wrap gap-2">
      {options.map((o) => (
        <Chip key={o.v} selected={sel(o.v)} onClick={() => onChange(o.v)} tone={tone}>
          {o.emoji && <span>{o.emoji}</span>}
          {o.label}
        </Chip>
      ))}
    </div>
  )
}

/* ───────── Surfaces ───────── */
export function Card({ children, className, onClick }: { children: ReactNode; className?: string; onClick?: () => void }) {
  return (
    <div onClick={onClick} className={cx('rounded-xl2 border border-line/70 bg-surface shadow-soft', onClick && 'press cursor-pointer', className)}>
      {children}
    </div>
  )
}

export function Section({ title, hint, children, right }: { title: string; hint?: string; children: ReactNode; right?: ReactNode }) {
  return (
    <section className="mb-6">
      <div className="mb-2.5 flex items-end justify-between px-1">
        <div>
          <h3 className="text-[15px] font-bold">{title}</h3>
          {hint && <p className="mt-0.5 text-xs text-muted">{hint}</p>}
        </div>
        {right}
      </div>
      {children}
    </section>
  )
}

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cx('h-6 w-6 animate-spin text-brand', className)} />
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cx('skeleton rounded-2xl', className)} />
}

export function FullSpinner({ label }: { label?: string }) {
  return (
    <div className="flex h-full min-h-[50vh] flex-col items-center justify-center gap-3 text-muted">
      <Spinner className="h-8 w-8" />
      {label && <p className="text-sm">{label}</p>}
    </div>
  )
}

export function EmptyState({ emoji, title, text, action }: { emoji: string; title: string; text?: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center px-8 py-14 text-center animate-rise">
      <div className="mb-4 flex h-20 w-20 items-center justify-center rounded-full bg-elevated text-4xl">{emoji}</div>
      <h3 className="text-lg font-bold">{title}</h3>
      {text && <p className="mt-1.5 max-w-xs text-sm text-muted">{text}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}

export function Avatar({ src, name, size = 48, className }: { src?: string; name?: string; size?: number; className?: string }) {
  const [bad, setBad] = useState(false)
  return (
    <div
      className={cx('shrink-0 overflow-hidden rounded-full bg-elevated', className)}
      style={{ width: size, height: size }}
    >
      {src && !bad ? (
        <img src={src} alt="" loading="lazy" className="h-full w-full object-cover" onError={() => setBad(true)} />
      ) : (
        <div className="brand-gradient flex h-full w-full items-center justify-center font-bold text-white" style={{ fontSize: size * 0.4 }}>
          {(name ?? '?').slice(0, 1).toUpperCase()}
        </div>
      )}
    </div>
  )
}

export function Switch({ checked, onChange, disabled }: { checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => { platform.haptic('select'); onChange(!checked) }}
      className={cx('relative h-7 w-12 shrink-0 rounded-full transition-colors disabled:opacity-50', checked ? 'brand-gradient' : 'bg-line')}
    >
      <span className={cx('absolute top-0.5 h-6 w-6 rounded-full bg-white shadow transition-all', checked ? 'left-[22px]' : 'left-0.5')} />
    </button>
  )
}

export function Row({ icon, title, sub, right, onClick, danger }: {
  icon?: ReactNode; title: string; sub?: string; right?: ReactNode; onClick?: () => void; danger?: boolean
}) {
  return (
    <div
      onClick={onClick}
      className={cx('flex items-center gap-3 px-4 py-3.5', onClick && 'press cursor-pointer active:bg-elevated/60')}
    >
      {icon && <div className={cx('flex h-9 w-9 items-center justify-center rounded-xl', danger ? 'bg-danger/10 text-danger' : 'bg-brand/10 text-brand')}>{icon}</div>}
      <div className="min-w-0 flex-1">
        <div className={cx('text-[15px] font-semibold', danger && 'text-danger')}>{title}</div>
        {sub && <div className="mt-0.5 truncate text-xs text-muted">{sub}</div>}
      </div>
      {right}
    </div>
  )
}

export function Group({ children }: { children: ReactNode }) {
  return <div className="mb-5 divide-y divide-line/70 overflow-hidden rounded-xl2 border border-line/70 bg-surface shadow-soft">{children}</div>
}

export function Progress({ value, className }: { value: number; className?: string }) {
  return (
    <div className={cx('h-1.5 w-full overflow-hidden rounded-full bg-line', className)}>
      <div className="brand-gradient h-full rounded-full transition-all duration-500" style={{ width: `${Math.min(100, Math.max(0, value))}%` }} />
    </div>
  )
}

export function Badge({ children, tone = 'brand' }: { children: ReactNode; tone?: 'brand' | 'friend' | 'ok' | 'muted' }) {
  const t = { brand: 'bg-brand/10 text-brand', friend: 'bg-friend/10 text-friend', ok: 'bg-ok/10 text-ok', muted: 'bg-elevated text-muted' }[tone]
  return <span className={cx('inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold', t)}>{children}</span>
}

export function Field({ label, hint, error, children }: { label?: string; hint?: string; error?: string; children: ReactNode }) {
  return (
    <label className="mb-4 block">
      {label && <span className="mb-1.5 block px-1 text-[13px] font-semibold text-muted">{label}</span>}
      {children}
      {error ? <span className="mt-1.5 block px-1 text-xs text-danger">{error}</span> : hint ? <span className="mt-1.5 block px-1 text-xs text-muted">{hint}</span> : null}
    </label>
  )
}

/* ───────── Bottom sheet ───────── */
export function Sheet({ open, onClose, title, children, footer, tall }: {
  open: boolean; onClose: () => void; title?: string; children: ReactNode; footer?: ReactNode; tall?: boolean
}) {
  useEffect(() => {
    if (!open) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [open])
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center" role="dialog" aria-modal="true">
      <div className="absolute inset-0 animate-fade bg-black/50 backdrop-blur-[2px]" onClick={onClose} />
      <div
        className={cx('relative flex w-full max-w-xl animate-sheet flex-col rounded-t-[28px] bg-bg shadow-card', tall ? 'h-[92dvh]' : 'max-h-[90dvh]')}
      >
        <div className="mx-auto mt-2.5 h-1.5 w-10 shrink-0 rounded-full bg-line" />
        {title ? (
          <div className="flex items-center justify-between px-5 pb-2 pt-3">
            <h2 className="text-lg font-extrabold">{title}</h2>
            <button onClick={onClose} className="press flex h-9 w-9 items-center justify-center rounded-full bg-elevated" aria-label="Close">
              <X className="h-5 w-5" />
            </button>
          </div>
        ) : (
          <button onClick={onClose} className="press absolute right-4 top-6 z-20 flex h-9 w-9 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur" aria-label="Close">
            <X className="h-5 w-5" />
          </button>
        )}
        <div className="scroll-hide flex-1 overflow-y-auto px-5 pb-4">{children}</div>
        {footer && <div className="border-t border-line/70 px-5 pt-3" style={{ paddingBottom: 'calc(12px + var(--safe-b))' }}>{footer}</div>}
        {!footer && <div style={{ height: 'var(--safe-b)' }} />}
      </div>
    </div>
  )
}

export function Modal({ open, onClose, children }: { open: boolean; onClose: () => void; children: ReactNode }) {
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-6">
      <div className="absolute inset-0 animate-fade bg-black/55 backdrop-blur-[2px]" onClick={onClose} />
      <div className="relative w-full max-w-sm animate-pop rounded-xl3 bg-surface p-6 shadow-card">{children}</div>
    </div>
  )
}

export function Confirm({ open, title, text, confirmLabel = 'Confirm', danger, onConfirm, onClose }: {
  open: boolean; title: string; text?: string; confirmLabel?: string; danger?: boolean; onConfirm: () => void | Promise<void>; onClose: () => void
}) {
  const [busy, setBusy] = useState(false)
  return (
    <Modal open={open} onClose={onClose}>
      <h3 className="text-lg font-extrabold">{title}</h3>
      {text && <p className="mt-2 text-sm text-muted">{text}</p>}
      <div className="mt-5 flex gap-3">
        <Button variant="soft" block onClick={onClose}>Cancel</Button>
        <Button
          variant={danger ? 'danger' : 'primary'}
          block
          loading={busy}
          onClick={async () => {
            setBusy(true)
            try { await onConfirm() } finally { setBusy(false) }
          }}
        >
          {confirmLabel}
        </Button>
      </div>
    </Modal>
  )
}

/* ───────── Toasts ───────── */
export function Toasts() {
  const toasts = useUI((s) => s.toasts)
  return (
    <div className="pointer-events-none fixed inset-x-0 top-0 z-[60] flex flex-col items-center gap-2 px-4" style={{ paddingTop: 'calc(12px + var(--safe-t))' }}>
      {toasts.map((t) => (
        <div
          key={t.id}
          className={cx(
            'pointer-events-auto flex max-w-sm animate-pop items-center gap-2 rounded-2xl px-4 py-3 text-sm font-semibold text-white shadow-card',
            t.kind === 'error' ? 'bg-danger' : t.kind === 'success' ? 'bg-ok' : 'bg-ink text-bg',
          )}
        >
          {t.kind === 'success' && <Check className="h-4 w-4" />}
          {t.text}
        </div>
      ))}
    </div>
  )
}

/* ───────── hooks ───────── */
export function useDebounced<T>(value: T, ms = 350): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

export function useInterval(fn: () => void, ms: number | null) {
  const ref = useRef(fn)
  ref.current = fn
  useEffect(() => {
    if (ms === null) return
    const id = setInterval(() => ref.current(), ms)
    return () => clearInterval(id)
  }, [ms])
}

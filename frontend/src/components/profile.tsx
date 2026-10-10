import { useMemo, useRef, useState } from 'react'
import { Ban, ChevronLeft, ChevronRight, Flag, MapPin, MoreHorizontal, ShieldCheck, X, Heart } from 'lucide-react'
import { VerifyBadge, VerifyPill } from '@/components/VerifyBadge'
import { post } from '@/api/client'
import type { Catalog, PublicProfile } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError } from '@/store/ui'
import { platform } from '@/platform/telegram'
import { Button, Chip, Confirm, Sheet, cx } from '@/components/ui'
import { LEVELS, LIFESTYLE, REPORT_REASONS, STRENGTH, flag, reasonText } from '@/lib/format'

/* ───────── small pieces ───────── */

export function IntentBadges({ types, className }: { types: string[]; className?: string }) {
  const both = types.includes('friends') && types.includes('relationship')
  return (
    <div className={cx('flex flex-wrap gap-1.5', className)}>
      {both ? (
        <span className="both-gradient inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-bold text-white">✨ Friends & Relationship</span>
      ) : (
        types.map((t) => (
          <span
            key={t}
            className={cx(
              'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-bold text-white',
              t === 'friends' ? 'friend-gradient' : t === 'relationship' ? 'love-gradient' : 'brand-gradient',
            )}
          >
            {t === 'friends' ? '🤝 Friends' : t === 'relationship' ? '❤️ Relationship' : t}
          </span>
        ))
      )}
    </div>
  )
}

/** Seals shown next to a name. Telegram is implied for everyone, so it is only spelled out in the full profile. */
export function VerifiedBadges({ p, size = 16 }: { p: PublicProfile; size?: number }) {
  const b = p.badges
  if (!b.identity && !b.photo && !b.phone && !b.premium) return null
  return (
    <span className="inline-flex items-center gap-1 align-middle">
      {b.identity && <VerifyBadge kind="identity" size={size} />}
      {b.photo && <VerifyBadge kind="photo" size={size} />}
      {b.phone && <VerifyBadge kind="phone" size={size} />}
      {b.premium && <VerifyBadge kind="plus" size={size} />}
    </span>
  )
}

function useNames(cat: Catalog | null) {
  return useMemo(() => {
    const i = new Map((cat?.interests ?? []).map((x) => [x.slug, x]))
    const l = new Map((cat?.languages ?? []).map((x) => [x.code, x]))
    return { interest: (s: string) => i.get(s), lang: (c: string) => l.get(c)?.name ?? c }
  }, [cat])
}

/* ───────── Photo viewer with tap zones ───────── */

export function PhotoStage({ photos, rounded = true, children }: { photos: PublicProfile['photos']; rounded?: boolean; children?: React.ReactNode }) {
  const [i, setI] = useState(0)
  const n = photos.length
  const go = (d: number, e: React.SyntheticEvent) => {
    e.stopPropagation()
    if (n < 2) return
    setI((x) => (x + d + n) % n)
    platform.haptic('select')
  }
  return (
    <div className={cx('relative h-full w-full overflow-hidden bg-elevated', rounded && 'rounded-xl3')}>
      {photos.length > 0 ? (
        <img key={photos[i].id} src={photos[i].url} alt="" draggable={false} className="h-full w-full select-none object-cover" />
      ) : (
        <div className="ph-bg h-full w-full" />
      )}
      {n > 1 && (
        <>
          <div className="absolute inset-x-3 top-3 flex gap-1.5">
            {photos.map((p, k) => (
              <span key={p.id} className={cx('h-1 flex-1 rounded-full', k === i ? 'bg-white' : 'bg-white/35')} />
            ))}
          </div>
          <button aria-label="Previous photo" className="absolute inset-y-0 left-0 w-1/3" onClick={(e) => go(-1, e)} />
          <button aria-label="Next photo" className="absolute inset-y-0 right-0 w-1/3" onClick={(e) => go(1, e)} />
        </>
      )}
      {children}
    </div>
  )
}

/* ───────── Discovery card ───────── */

export function ProfileCard({ p, onInfo }: { p: PublicProfile; onInfo: () => void }) {
  const cat = useSession((s) => s.catalog)
  const names = useNames(cat)
  const strength = p.compat ? STRENGTH[p.compat.strength] : null
  const top = p.compat?.reasons.find((r) => r.type === 'interests')?.items ?? []
  const shown = [...top, ...p.interests.filter((x) => !top.includes(x))].slice(0, 4)

  return (
    <PhotoStage photos={p.photos}>
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-3/5 bg-gradient-to-t from-black/85 via-black/40 to-transparent" />
      {strength && (
        <span className={cx('absolute left-4 top-8 rounded-full px-3 py-1 text-xs font-bold shadow-soft backdrop-blur', strength.cls)}>
          {p.liked_you ? '💘 Likes you · ' : ''}{strength.label}
        </span>
      )}
      <div className="pointer-events-none absolute inset-x-0 bottom-0 p-5 text-white">
        <IntentBadges types={p.connection_types} className="mb-2" />
        <div className="flex items-end justify-between gap-3">
          <div className="min-w-0">
            <h2 className="truncate text-[30px] font-extrabold leading-tight drop-shadow">
              {p.display_name}, <span className="font-bold">{p.age}</span> <VerifiedBadges p={p} size={20} />
            </h2>
            <p className="mt-0.5 flex items-center gap-1 text-[15px] font-medium text-white/90">
              <MapPin className="h-4 w-4" /> {p.city}
              {p.area ? ` · ${p.area}` : ''} <span className="ml-0.5">{flag(p.country_code)}</span>
            </p>
          </div>
        </div>
        {shown.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {shown.map((s) => {
              const it = names.interest(s)
              return (
                <span key={s} className="rounded-full bg-white/20 px-2.5 py-1 text-xs font-semibold backdrop-blur-md">
                  {it?.emoji} {it?.name ?? s}
                </span>
              )
            })}
          </div>
        )}
      </div>
      <button
        onClick={(e) => { e.stopPropagation(); onInfo() }}
        onPointerDown={(e) => e.stopPropagation()}
        className="press absolute bottom-5 right-5 flex h-10 w-10 items-center justify-center rounded-full bg-white/25 text-white backdrop-blur-md"
        aria-label="More info"
      >
        <ChevronRight className="-rotate-90 h-6 w-6" />
      </button>
    </PhotoStage>
  )
}

/* ───────── Full profile sheet ───────── */

export function ProfileSheet({ p, onClose, actions, onChanged }: {
  p: PublicProfile | null
  onClose: () => void
  actions?: React.ReactNode
  /** called after block / report so the parent can remove the profile */
  onChanged?: (kind: 'blocked' | 'reported') => void
}) {
  const cat = useSession((s) => s.catalog)
  const names = useNames(cat)
  const [menu, setMenu] = useState(false)
  const [report, setReport] = useState(false)
  const [block, setBlock] = useState(false)
  const scroller = useRef<HTMLDivElement>(null)
  if (!p) return null

  const shared = new Set(p.compat?.reasons.find((r) => r.type === 'interests')?.items ?? [])
  const reasons = (p.compat?.reasons ?? []).map((r) => reasonText(r, cat)).filter(Boolean)
  const strength = p.compat ? STRENGTH[p.compat.strength] : null

  return (
    <Sheet open onClose={onClose} tall footer={actions}>
      <div ref={scroller}>
        <div className="-mx-1 mb-4 aspect-[4/5] overflow-hidden rounded-xl3 shadow-card">
          <PhotoStage photos={p.photos} />
        </div>

        <div className="mb-1 flex items-start justify-between">
          <div>
            <h2 className="text-[26px] font-extrabold leading-tight">
              {p.display_name}, {p.age} <VerifiedBadges p={p} size={20} />
            </h2>
            <p className="mt-0.5 flex items-center gap-1 text-sm text-muted">
              <MapPin className="h-4 w-4" /> {p.city}{p.area ? ` · ${p.area}` : ''} {flag(p.country_code)}
              {p.gender && <span className="ml-1">· {p.gender.replace('_', '-')}</span>}
            </p>
            <p className="mt-0.5 text-xs text-faint">{p.atish_username}</p>
          </div>
          <div className="relative">
            <button onClick={() => setMenu((m) => !m)} className="press flex h-10 w-10 items-center justify-center rounded-full bg-elevated" aria-label="More">
              <MoreHorizontal className="h-5 w-5" />
            </button>
            {menu && (
              <div className="absolute right-0 top-11 z-10 w-44 animate-pop overflow-hidden rounded-2xl border border-line bg-surface shadow-card">
                <button className="press flex w-full items-center gap-2 px-4 py-3 text-sm font-semibold" onClick={() => { setMenu(false); setReport(true) }}>
                  <Flag className="h-4 w-4" /> Report
                </button>
                <button className="press flex w-full items-center gap-2 px-4 py-3 text-sm font-semibold text-danger" onClick={() => { setMenu(false); setBlock(true) }}>
                  <Ban className="h-4 w-4" /> Block
                </button>
              </div>
            )}
          </div>
        </div>

        <IntentBadges types={p.connection_types} className="my-3" />

        <div className="mb-4 flex flex-wrap gap-1.5">
          {p.badges.telegram && <VerifyPill kind="telegram" label="Telegram verified" />}
          {p.badges.phone && <VerifyPill kind="phone" label="Phone verified" />}
          {p.badges.photo && <VerifyPill kind="photo" label="Photo verified" />}
          {p.badges.identity && <VerifyPill kind="identity" label="ID verified" />}
          {p.badges.premium && <VerifyPill kind="plus" label="Atish Plus" />}
        </div>

        {p.compat && reasons.length > 0 && (
          <div className="mb-5 rounded-xl2 border border-line/70 bg-surface p-4 shadow-soft">
            <div className="mb-2 flex items-center gap-2">
              {strength && <span className={cx('rounded-full px-2.5 py-0.5 text-xs font-bold', strength.cls)}>{strength.label}</span>}
              <span className="text-sm font-bold">Why you might click</span>
            </div>
            <ul className="space-y-1.5">
              {reasons.map((r) => (
                <li key={r} className="flex items-start gap-2 text-sm">
                  <span className="mt-0.5 text-ok">✓</span> {r}
                </li>
              ))}
            </ul>
          </div>
        )}

        {p.bio && <p className="mb-5 whitespace-pre-line text-[15px] leading-relaxed">{p.bio}</p>}

        {p.friendship_kinds.length > 0 && (
          <Block title="Into">
            {p.friendship_kinds.map((k) => {
              const it = cat?.friendship_kinds.find((x) => x.slug === k)
              return <Chip key={k} tone="friend" selected>{it?.emoji} {it?.name ?? k}</Chip>
            })}
          </Block>
        )}

        {p.interests.length > 0 && (
          <Block title="Interests">
            {p.interests.map((s) => {
              const it = names.interest(s)
              return (
                <Chip key={s} selected={shared.has(s)} tone="brand">
                  {it?.emoji} {it?.name ?? s}
                </Chip>
              )
            })}
          </Block>
        )}

        {p.languages.length > 0 && (
          <Block title="Languages">
            {p.languages.map((l) => (
              <Chip key={l.code} tone="muted" selected>
                {names.lang(l.code)} <span className="text-xs text-muted">· {LEVELS.find((x) => x.v === l.level)?.label}</span>
                {l.wants_practice && <span title="Wants to practice">🗣️</span>}
              </Chip>
            ))}
          </Block>
        )}

        {p.occupation && (p.occupation.profession || p.occupation.university || p.occupation.occupation_status) && (
          <Block title="Work & education">
            {[p.occupation.profession, p.occupation.industry, p.occupation.university, p.occupation.field_of_study, p.occupation.degree]
              .filter(Boolean)
              .map((t) => <Chip key={t} tone="muted" selected>{t}</Chip>)}
          </Block>
        )}

        {p.lifestyle && (
          <Block title="Lifestyle">
            {LIFESTYLE.map((l) => {
              const v = (p.lifestyle as unknown as Record<string, string>)[l.key]
              const opt = l.options.find((o) => o.v === v)
              return opt ? <Chip key={l.key} tone="muted" selected>{l.label}: {opt.label}</Chip> : null
            })}
          </Block>
        )}

        <p className="mb-2 mt-6 flex items-center justify-center gap-1.5 text-xs text-faint">
          <ShieldCheck className="h-3.5 w-3.5" /> Exact location and contact details are never shared
        </p>
      </div>

      <ReportSheet open={report} onClose={() => setReport(false)} userId={p.id} name={p.display_name} onDone={() => { onChanged?.('reported') }} />
      <Confirm
        open={block}
        danger
        title={`Block ${p.display_name}?`}
        text="They won't be able to see or contact you, and you won't see them. You can unblock later in Settings."
        confirmLabel="Block"
        onClose={() => setBlock(false)}
        onConfirm={async () => {
          try {
            await post(`/api/users/${p.id}/block`)
            toast(`${p.display_name} was blocked`, 'success')
            setBlock(false)
            onChanged?.('blocked')
          } catch (e) { toastError(e) }
        }}
      />
    </Sheet>
  )
}

function Block({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mb-5">
      <h4 className="mb-2 text-xs font-bold uppercase tracking-wide text-muted">{title}</h4>
      <div className="flex flex-wrap gap-2">{children}</div>
    </div>
  )
}

/* ───────── Report ───────── */

export function ReportSheet({ open, onClose, userId, name, onDone }: { open: boolean; onClose: () => void; userId: string; name: string; onDone?: () => void }) {
  const [reason, setReason] = useState('')
  const [details, setDetails] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <Sheet
      open={open}
      onClose={onClose}
      title={`Report ${name}`}
      footer={
        <Button block variant="danger" disabled={!reason} loading={busy} onClick={async () => {
          setBusy(true)
          try {
            await post(`/api/users/${userId}/report`, { reason, details })
            toast('Thanks — our team will review this', 'success')
            setReason(''); setDetails('')
            onClose(); onDone?.()
          } catch (e) { toastError(e) } finally { setBusy(false) }
        }}>
          Send report
        </Button>
      }
    >
      <p className="mb-3 text-sm text-muted">Your report is private. The person won't know it was you.</p>
      <div className="mb-4 space-y-2">
        {REPORT_REASONS.map((r) => (
          <button
            key={r.v}
            onClick={() => setReason(r.v)}
            className={cx('press flex w-full items-center justify-between rounded-2xl border px-4 py-3.5 text-left font-semibold', reason === r.v ? 'border-brand bg-brand/10 text-brand' : 'border-line bg-surface')}
          >
            {r.label}
            {reason === r.v && <span>✓</span>}
          </button>
        ))}
      </div>
      <textarea className="field min-h-[90px] resize-none" maxLength={1000} placeholder="Add details (optional)" value={details} onChange={(e) => setDetails(e.target.value)} />
    </Sheet>
  )
}

/* ───────── Match celebration ───────── */

export function MatchOverlay({ other, me, onChat, onClose }: { other: PublicProfile; me: string | undefined; onChat: () => void; onClose: () => void }) {
  const friendsOnly = other.compat?.types.length === 1 && other.compat.types[0] === 'friends'
  const hearts = useMemo(() => Array.from({ length: 14 }, (_, i) => ({ i, left: Math.random() * 100, delay: Math.random() * 1.2, size: 16 + Math.random() * 22 })), [])
  return (
    <div className="fixed inset-0 z-[55] flex flex-col items-center justify-center overflow-hidden brand-gradient p-8 text-center text-white animate-fade">
      {hearts.map((h) => (
        <span key={h.i} className="pointer-events-none absolute bottom-0 animate-float" style={{ left: `${h.left}%`, fontSize: h.size, animationDelay: `${h.delay}s`, animationDuration: `${2.6 + Math.random() * 2}s` }}>
          {friendsOnly ? '🤝' : '❤️'}
        </span>
      ))}
      <div className="relative mb-8 flex items-center">
        <div className="-mr-6 h-36 w-36 -rotate-6 overflow-hidden rounded-3xl border-4 border-white shadow-card">
          <MyPhoto />
        </div>
        <div className="h-36 w-36 rotate-6 overflow-hidden rounded-3xl border-4 border-white shadow-card">
          {other.photos[0] && <img src={other.photos[0].url} alt="" className="h-full w-full object-cover" />}
        </div>
        <span className="absolute left-1/2 top-1/2 flex h-14 w-14 -translate-x-1/2 -translate-y-1/2 animate-heart items-center justify-center rounded-full bg-white text-2xl shadow-card">
          {friendsOnly ? '🤝' : '🔥'}
        </span>
      </div>
      <h1 className="text-4xl font-black drop-shadow">It's a match!</h1>
      <p className="mt-2 max-w-xs text-white/90">
        You and <b>{other.display_name}</b> {friendsOnly ? 'both want to be friends.' : 'liked each other.'} Say hello!
      </p>
      <div className="mt-8 flex w-full max-w-xs flex-col gap-3">
        <button onClick={onChat} className="press h-14 rounded-2xl bg-white text-base font-extrabold text-brand shadow-card">Send a message</button>
        <button onClick={onClose} className="press h-12 rounded-2xl bg-white/20 font-semibold backdrop-blur">Keep exploring</button>
      </div>
      <span className="hidden">{me}</span>
    </div>
  )
}

function MyPhoto() {
  const me = useSession((s) => s.me)
  return me?.photos[0] ? <img src={me.photos[0].url} alt="" className="h-full w-full object-cover" /> : <div className="ph-bg h-full w-full" />
}

/* ───────── Like / pass buttons ───────── */

export function ActionButtons({ onPass, onLike, onRewind, canRewind, disabled }: {
  onPass: () => void; onLike: () => void; onRewind?: () => void; canRewind?: boolean; disabled?: boolean
}) {
  return (
    <div className="flex items-center justify-center gap-6">
      {onRewind && (
        <button onClick={onRewind} disabled={disabled} className="press flex h-12 w-12 items-center justify-center rounded-full bg-surface text-amber-500 shadow-soft disabled:opacity-40" aria-label="Rewind">
          <ChevronLeft className="h-6 w-6" />
          {!canRewind && <span className="absolute text-[8px]">★</span>}
        </button>
      )}
      <button onClick={onPass} disabled={disabled} className="press flex h-[68px] w-[68px] items-center justify-center rounded-full border-2 border-line bg-surface text-muted shadow-card disabled:opacity-40" aria-label="Not interested">
        <X className="h-9 w-9" strokeWidth={3} />
      </button>
      <button onClick={onLike} disabled={disabled} className="press brand-gradient flex h-[76px] w-[76px] items-center justify-center rounded-full text-white shadow-glow disabled:opacity-40" aria-label="Interested">
        <Heart className="h-9 w-9" fill="currentColor" />
      </button>
    </div>
  )
}


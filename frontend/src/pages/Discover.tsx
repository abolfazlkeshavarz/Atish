import { Logo } from '@/components/Logo'
import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { SlidersHorizontal, RefreshCw, Undo2 } from 'lucide-react'
import { create } from 'zustand'
import { ApiError, get, patch, post } from '@/api/client'
import type { FeedResponse, Me, PublicProfile, SwipeResult } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError, useUI } from '@/store/ui'
import { platform } from '@/platform/telegram'
import { Button, Chip, ChipGroup, EmptyState, Sheet, Skeleton, cx } from '@/components/ui'
import { ActionButtons, MatchOverlay, ProfileCard, ProfileSheet } from '@/components/profile'
import { AgeRange } from '@/components/editors'
import { SCOPES } from '@/lib/format'

/* ───────── filters (kept on this device) ───────── */

interface Filters { type: string; ageMin: number; ageMax: number; interests: string[]; language: string }
const DEFAULT_FILTERS: Filters = { type: '', ageMin: 0, ageMax: 0, interests: [], language: '' }
const FKEY = 'atish.filters'

const useFilters = create<{ f: Filters; set(f: Filters): void }>((set) => {
  let init = DEFAULT_FILTERS
  try { init = { ...DEFAULT_FILTERS, ...JSON.parse(localStorage.getItem(FKEY) || '{}') } } catch { /* ignore */ }
  return {
    f: init,
    set: (f) => {
      try { localStorage.setItem(FKEY, JSON.stringify(f)) } catch { /* ignore */ }
      set({ f })
    },
  }
})

const activeFilterCount = (f: Filters) => (f.type ? 1 : 0) + (f.ageMin || f.ageMax ? 1 : 0) + (f.interests.length ? 1 : 0) + (f.language ? 1 : 0)

/* ───────── swipeable card ───────── */

interface CardHandle { fly(dir: 'left' | 'right'): void }
const THRESHOLD = 110

const SwipeCard = forwardRef<CardHandle, { p: PublicProfile; onSwipe: (dir: 'left' | 'right') => void; onInfo: () => void }>(function SwipeCard({ p, onSwipe, onInfo }, ref) {
  const [pos, setPos] = useState({ x: 0, y: 0, dragging: false })
  const start = useRef<{ x: number; y: number; t: number } | null>(null)
  const done = useRef(false)

  const fly = useCallback((dir: 'left' | 'right') => {
    if (done.current) return
    done.current = true
    platform.haptic(dir === 'right' ? 'medium' : 'light')
    setPos({ x: dir === 'right' ? window.innerWidth * 1.3 : -window.innerWidth * 1.3, y: 60, dragging: false })
    setTimeout(() => onSwipe(dir), 240)
  }, [onSwipe])

  useImperativeHandle(ref, () => ({ fly }), [fly])

  const x = pos.x
  const likeOpacity = Math.min(1, Math.max(0, x / 90))
  const nopeOpacity = Math.min(1, Math.max(0, -x / 90))

  return (
    <div
      className="absolute inset-0 select-none"
      style={{
        transform: `translate(${pos.x}px, ${pos.y}px) rotate(${pos.x / 18}deg)`,
        transition: pos.dragging ? 'none' : 'transform .28s cubic-bezier(.2,.8,.2,1)',
        touchAction: 'none',
        willChange: 'transform',
      }}
      onPointerDown={(e) => {
        if (done.current) return
        ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
        start.current = { x: e.clientX, y: e.clientY, t: Date.now() }
      }}
      onPointerMove={(e) => {
        const s = start.current
        if (!s || done.current) return
        setPos({ x: e.clientX - s.x, y: (e.clientY - s.y) * 0.35, dragging: true })
      }}
      onPointerUp={(e) => {
        const s = start.current
        start.current = null
        if (!s || done.current) return
        const dx = e.clientX - s.x
        const v = Math.abs(dx) / Math.max(1, Date.now() - s.t)
        if (dx > THRESHOLD || (dx > 40 && v > 0.7)) fly('right')
        else if (dx < -THRESHOLD || (dx < -40 && v > 0.7)) fly('left')
        else setPos({ x: 0, y: 0, dragging: false })
      }}
      onPointerCancel={() => { start.current = null; setPos({ x: 0, y: 0, dragging: false }) }}
    >
      <div className="h-full w-full shadow-card [border-radius:1.75rem]">
        <ProfileCard p={p} onInfo={onInfo} />
      </div>
      <div className="pointer-events-none absolute left-6 top-20 -rotate-12 rounded-xl border-4 border-ok px-3 py-1 text-3xl font-black tracking-wider text-ok" style={{ opacity: likeOpacity }}>
        LIKE
      </div>
      <div className="pointer-events-none absolute right-6 top-20 rotate-12 rounded-xl border-4 border-danger px-3 py-1 text-3xl font-black tracking-wider text-danger" style={{ opacity: nopeOpacity }}>
        NOPE
      </div>
    </div>
  )
})

/* ───────── page ───────── */

export default function Discover() {
  const nav = useNavigate()
  const { me, catalog, setMe } = useSession()
  const openPremium = useUI((s) => s.openPremium)
  const { f: filters, set: setFilters } = useFilters()

  const [queue, setQueue] = useState<PublicProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [exhausted, setExhausted] = useState(false)
  const [likesLeft, setLikesLeft] = useState<number | null>(null)
  const [info, setInfo] = useState<PublicProfile | null>(null)
  const [match, setMatch] = useState<{ user: PublicProfile; id: string } | null>(null)
  const [filterOpen, setFilterOpen] = useState(false)
  const [error, setError] = useState('')

  const seen = useRef(new Set<string>())
  const fetching = useRef(false)
  const top = useRef<CardHandle>(null)

  const load = useCallback(async (reset = false) => {
    if (fetching.current) return
    fetching.current = true
    setLoading(true)
    setError('')
    try {
      if (reset) { seen.current = new Set(); setQueue([]); setExhausted(false) }
      const r = await get<FeedResponse>('/api/discover', {
        type: filters.type, min_age: filters.ageMin, max_age: filters.ageMax,
        interests: filters.interests.join(','), language: filters.language,
      })
      setLikesLeft(r.likes_left)
      const fresh = r.profiles.filter((p) => !seen.current.has(p.id))
      fresh.forEach((p) => seen.current.add(p.id))
      setQueue((q) => [...q, ...fresh])
      if (fresh.length === 0) setExhausted(true)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      fetching.current = false
      setLoading(false)
    }
  }, [filters])

  // (re)load whenever filters change
  useEffect(() => { void load(true) }, [load])

  // prefetch before the deck runs dry
  useEffect(() => {
    if (queue.length <= 2 && !loading && !exhausted && !error) void load()
  }, [queue.length, loading, exhausted, error, load])

  const decide = async (p: PublicProfile, dir: 'left' | 'right') => {
    setQueue((q) => q.filter((x) => x.id !== p.id))
    setInfo(null)
    if (dir === 'left') {
      post(`/api/discover/${p.id}/pass`).catch(() => {})
      return
    }
    try {
      const r = await post<SwipeResult>(`/api/discover/${p.id}/like`)
      setLikesLeft(r.likes_left)
      if (r.matched && r.user && r.match_id) {
        platform.haptic('success')
        setMatch({ user: r.user, id: r.match_id })
      }
    } catch (e) {
      const err = e as ApiError
      if (err.code === 'daily_limit') {
        setQueue((q) => [p, ...q])
        setLikesLeft(0)
        openPremium('limit')
      } else if (err.code === 'not_found' || err.code === 'not_compatible') {
        /* profile went away — skip silently */
      } else {
        toastError(e)
      }
    }
  }

  const rewind = async () => {
    if (!me?.entitlements.features.rewind) return openPremium('rewind')
    try {
      const p = await post<PublicProfile>('/api/discover/rewind')
      seen.current.add(p.id)
      setQueue((q) => [p, ...q.filter((x) => x.id !== p.id)])
      platform.haptic('light')
    } catch (e) { toast((e as Error).message) }
  }

  const current = queue[0]
  const next = queue[1]

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center justify-between px-4 pb-2" style={{ paddingTop: 'calc(10px + var(--safe-t))' }}>
        <div className="flex items-center gap-2">
          <Logo tile size={36} />
          <span className="text-gradient text-2xl font-black tracking-tight">Atish</span>
        </div>
        <div className="flex items-center gap-2">
          {likesLeft !== null && (
            <button onClick={() => openPremium('limit')} className="press rounded-full bg-surface px-3 py-1.5 text-xs font-bold shadow-soft">
              ❤️ {likesLeft} left
            </button>
          )}
          <button onClick={() => setFilterOpen(true)} className="press relative flex h-10 w-10 items-center justify-center rounded-full bg-surface shadow-soft" aria-label="Filters">
            <SlidersHorizontal className="h-5 w-5" />
            {activeFilterCount(filters) > 0 && (
              <span className="brand-gradient absolute -right-0.5 -top-0.5 flex h-4 w-4 items-center justify-center rounded-full text-[10px] font-bold text-white">{activeFilterCount(filters)}</span>
            )}
          </button>
        </div>
      </header>

      <div className="relative mx-3 mt-1 min-h-0 flex-1">
        {current ? (
          <>
            {next && (
              <div key={next.id} className="absolute inset-0 origin-bottom scale-[0.95] opacity-90">
                <div className="h-full w-full [border-radius:1.75rem] overflow-hidden shadow-soft"><ProfileCard p={next} onInfo={() => {}} /></div>
              </div>
            )}
            <SwipeCard key={current.id} ref={top} p={current} onSwipe={(d) => decide(current, d)} onInfo={() => setInfo(current)} />
          </>
        ) : loading ? (
          <Skeleton className="absolute inset-0 !rounded-[1.75rem]" />
        ) : error ? (
          <EmptyState emoji="📡" title="Couldn't load people" text={error} action={<Button onClick={() => load(true)}>Try again</Button>} />
        ) : (
          <EmptyState
            emoji="🌙"
            title="You're all caught up"
            text="No new people match your preferences right now. Check back soon or widen your search."
            action={
              <div className="flex flex-col gap-2">
                <Button onClick={() => load(true)}><RefreshCw className="h-4 w-4" /> Refresh</Button>
                <Button variant="soft" onClick={() => setFilterOpen(true)}>Adjust filters</Button>
              </div>
            }
          />
        )}
      </div>

      <div className="px-4 pb-3 pt-4">
        <ActionButtons
          disabled={!current}
          onPass={() => top.current?.fly('left')}
          onLike={() => top.current?.fly('right')}
        />
        {current && (
          <button onClick={rewind} className="press mx-auto mt-3 flex items-center gap-1.5 text-xs font-semibold text-muted">
            <Undo2 className="h-3.5 w-3.5" /> Undo last pass {!me?.entitlements.features.rewind && '· Plus'}
          </button>
        )}
      </div>

      <ProfileSheet
        p={info}
        onClose={() => setInfo(null)}
        onChanged={() => { if (info) { setQueue((q) => q.filter((x) => x.id !== info.id)); setInfo(null) } }}
        actions={
          info && (
            <div className="flex gap-3">
              <Button variant="outline" size="lg" block onClick={() => decide(info, 'left')}>✕ Not now</Button>
              <Button size="lg" block onClick={() => decide(info, 'right')}>❤️ Interested</Button>
            </div>
          )
        }
      />

      {match && (
        <MatchOverlay other={match.user} me={me?.id} onClose={() => setMatch(null)} onChat={() => { const id = match.id; setMatch(null); nav(`/chats/${id}`) }} />
      )}

      <FilterSheet
        open={filterOpen}
        onClose={() => setFilterOpen(false)}
        filters={filters}
        onApply={(nf) => { setFilters(nf); setFilterOpen(false) }}
        me={me}
        onScope={async (scope) => { if (me) setMe(await patch<Me>('/api/me/preferences', { preferences: { ...me.preferences, distance_scope: scope } })); void load(true) }}
        catalog={catalog}
      />
    </div>
  )
}

function FilterSheet({ open, onClose, filters, onApply, me, onScope, catalog }: {
  open: boolean; onClose: () => void; filters: Filters; onApply: (f: Filters) => void; me: Me | null; onScope: (s: string) => void
  catalog: ReturnType<typeof useSession.getState>['catalog']
}) {
  const [f, setF] = useState(filters)
  useEffect(() => { if (open) setF(filters) }, [open, filters])
  if (!me || !catalog) return null
  const minAge = 18
  const lo = f.ageMin || me.preferences.age_min
  const hi = f.ageMax || me.preferences.age_max
  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="Filters"
      tall
      footer={
        <div className="flex gap-3">
          <Button variant="soft" block onClick={() => onApply(DEFAULT_FILTERS)}>Reset</Button>
          <Button block onClick={() => onApply(f)}>Show results</Button>
        </div>
      }
    >
      <div className="space-y-6 pb-2">
        <div>
          <p className="mb-2 text-[13px] font-bold uppercase tracking-wide text-muted">Looking for</p>
          <div className="flex flex-wrap gap-2">
            <Chip selected={!f.type} onClick={() => setF({ ...f, type: '' })}>✨ Everything</Chip>
            {catalog.connection_types.filter((t) => me.connection_types.includes(t.slug)).map((t) => (
              <Chip key={t.slug} selected={f.type === t.slug} tone={t.slug === 'friends' ? 'friend' : 'love'} onClick={() => setF({ ...f, type: t.slug })}>
                {t.emoji} {t.name}
              </Chip>
            ))}
          </div>
        </div>
        <AgeRange min={lo} max={hi} floor={minAge} onChange={(a, b) => setF({ ...f, ageMin: a, ageMax: b })} />
        <div>
          <p className="mb-2 text-[13px] font-bold uppercase tracking-wide text-muted">Distance</p>
          <ChipGroup options={SCOPES} value={me.preferences.distance_scope} onChange={onScope} />
        </div>
        <div>
          <p className="mb-2 text-[13px] font-bold uppercase tracking-wide text-muted">Speaks</p>
          <select className="field" value={f.language} onChange={(e) => setF({ ...f, language: e.target.value })}>
            <option value="">Any language</option>
            {catalog.languages.map((l) => <option key={l.code} value={l.code}>{l.name}</option>)}
          </select>
        </div>
        <div>
          <p className="mb-2 text-[13px] font-bold uppercase tracking-wide text-muted">Shared interests</p>
          <div className="flex flex-wrap gap-2">
            {catalog.interests.map((i) => (
              <Chip key={i.slug} size="sm" selected={f.interests.includes(i.slug)}
                onClick={() => setF({ ...f, interests: f.interests.includes(i.slug) ? f.interests.filter((x) => x !== i.slug) : [...f.interests, i.slug].slice(0, 5) })}>
                {i.emoji} {i.name}
              </Chip>
            ))}
          </div>
          <p className={cx('mt-2 text-xs text-muted')}>Pick up to 5. People who like any of them appear.</p>
        </div>
      </div>
    </Sheet>
  )
}

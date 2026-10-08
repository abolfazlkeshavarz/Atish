import { useEffect, useMemo, useRef, useState } from 'react'
import { Camera, Plus, X, Search, MapPin, Check, AlertCircle } from 'lucide-react'
import { api, del, get, post } from '@/api/client'
import type { Catalog, Loc, Me, UserLanguage } from '@/api/types'
import { useSession } from '@/store/session'
import { toastError } from '@/store/ui'
import { platform } from '@/platform/telegram'
import { Button, Chip, ChipGroup, Spinner, cx, useDebounced } from '@/components/ui'
import { COUNTRY_CODES, INTENTIONS, LEVELS, PREF_GENDERS, SCOPES, countryName, flag } from '@/lib/format'

/* ───────────────── Photos ───────────────── */

/** Downscales large camera photos in the browser so uploads are fast on mobile data. */
async function prepareImage(file: File): Promise<Blob> {
  if (file.size < 900_000) return file
  try {
    const bmp = await createImageBitmap(file)
    const scale = Math.min(1, 1600 / Math.max(bmp.width, bmp.height))
    const c = document.createElement('canvas')
    c.width = Math.round(bmp.width * scale)
    c.height = Math.round(bmp.height * scale)
    c.getContext('2d')!.drawImage(bmp, 0, 0, c.width, c.height)
    return await new Promise<Blob>((res, rej) => c.toBlob((b) => (b ? res(b) : rej()), 'image/jpeg', 0.86))
  } catch {
    return file
  }
}

export function PhotoManager({ hero }: { hero?: boolean }) {
  const { me, app, refresh } = useSession()
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const photos = me?.photos ?? []
  const max = app?.max_photos ?? 3
  const canTelegram = platform.inTelegram() || !!platform.prefill()?.photo_url

  const upload = async (file: File) => {
    setBusy(true)
    try {
      const blob = await prepareImage(file)
      const form = new FormData()
      form.append('photo', blob, 'photo.jpg')
      await api('POST', '/api/me/photos', { form })
      await refresh()
      platform.haptic('success')
    } catch (e) {
      toastError(e)
    } finally {
      setBusy(false)
    }
  }

  const importTelegram = async () => {
    setBusy(true)
    try {
      await post('/api/me/photos/telegram')
      await refresh()
      platform.haptic('success')
    } catch (e) {
      toastError(e)
    } finally {
      setBusy(false)
    }
  }

  const remove = async (id: string) => {
    setBusy(true)
    try {
      await del(`/api/me/photos/${id}`)
      await refresh()
    } catch (e) {
      toastError(e)
    } finally {
      setBusy(false)
    }
  }

  const slots = Array.from({ length: max }, (_, i) => photos[i] ?? null)
  const pick = () => input.current?.click()

  return (
    <div>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        hidden
        onChange={(e) => {
          const f = e.target.files?.[0]
          e.target.value = ''
          if (f) void upload(f)
        }}
      />
      {hero && photos.length === 0 ? (
        <div className="flex flex-col items-center">
          <button onClick={pick} className="press brand-gradient relative flex h-56 w-56 items-center justify-center rounded-full p-1 shadow-glow" disabled={busy}>
            <span className="flex h-full w-full flex-col items-center justify-center gap-2 rounded-full bg-surface text-muted">
              {busy ? <Spinner className="h-9 w-9" /> : <Camera className="h-12 w-12 text-brand" />}
              <span className="text-sm font-semibold">{busy ? 'Uploading…' : 'Add your best photo'}</span>
            </span>
          </button>
          {canTelegram && (
            <Button variant="soft" className="mt-6" onClick={importTelegram} loading={busy}>
              ✈️ Use my Telegram photo
            </Button>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-3">
          {slots.map((p, i) =>
            p ? (
              <div key={p.id} className={cx('relative aspect-[3/4] overflow-hidden rounded-2xl bg-elevated shadow-soft', i === 0 && 'ring-2 ring-brand')}>
                <img src={p.url} alt="" className="h-full w-full object-cover" />
                {i === 0 && <span className="brand-gradient absolute bottom-1.5 left-1.5 rounded-full px-2 py-0.5 text-[10px] font-bold text-white">MAIN</span>}
                <button
                  onClick={() => remove(p.id)}
                  disabled={busy}
                  className="press absolute right-1.5 top-1.5 flex h-7 w-7 items-center justify-center rounded-full bg-black/60 text-white"
                  aria-label="Remove photo"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            ) : (
              <button
                key={i}
                onClick={pick}
                disabled={busy}
                className="press flex aspect-[3/4] items-center justify-center rounded-2xl border-2 border-dashed border-line text-faint"
              >
                {busy && i === photos.length ? <Spinner /> : <Plus className="h-7 w-7" />}
              </button>
            ),
          )}
        </div>
      )}
    </div>
  )
}

/* ───────────────── Age range ───────────────── */

export function AgeRange({ min, max, onChange, floor = 18 }: { min: number; max: number; floor?: number; onChange: (min: number, max: number) => void }) {
  return (
    <div>
      <div className="mb-3 flex items-baseline justify-between">
        <span className="text-[13px] font-semibold text-muted">Age range</span>
        <span className="text-xl font-extrabold">
          {min} – {max >= 99 ? '99+' : max}
        </span>
      </div>
      <div className="space-y-4">
        <input type="range" min={floor} max={99} value={min} onChange={(e) => onChange(Math.min(+e.target.value, max), max)} aria-label="Minimum age" />
        <input type="range" min={floor} max={99} value={max} onChange={(e) => onChange(min, Math.max(+e.target.value, min))} aria-label="Maximum age" />
      </div>
    </div>
  )
}

/* ───────────────── Connection intent ───────────────── */

export interface IntentState {
  types: string[]
  kinds: string[]
  intention: string
  genders: string[]
  ageMin: number
  ageMax: number
  scope: string
}

export const intentFromMe = (me: Me): IntentState => ({
  types: me.connection_types,
  kinds: me.friendship_kinds,
  intention: me.preferences.relationship_intention,
  genders: me.preferences.preferred_genders,
  ageMin: me.preferences.age_min,
  ageMax: me.preferences.age_max,
  scope: me.preferences.distance_scope,
})

export function IntentEditor({ value, onChange, catalog, minAge }: { value: IntentState; onChange: (v: IntentState) => void; catalog: Catalog; minAge: number }) {
  const types = catalog.connection_types
  const allSelected = types.length > 1 && types.every((t) => value.types.includes(t.slug))
  const set = (p: Partial<IntentState>) => onChange({ ...value, ...p })

  const card = (key: string, emoji: string, title: string, sub: string, active: boolean, grad: string, click: () => void) => (
    <button
      key={key}
      onClick={() => { platform.haptic('medium'); click() }}
      className={cx(
        'press relative flex w-full items-center gap-4 overflow-hidden rounded-xl2 border-2 p-4 text-left transition',
        active ? 'border-transparent text-white shadow-card' : 'border-line bg-surface',
      )}
    >
      {active && <span className={cx('absolute inset-0', grad)} />}
      <span className={cx('relative flex h-14 w-14 items-center justify-center rounded-2xl text-3xl', active ? 'bg-white/20' : 'bg-elevated')}>{emoji}</span>
      <span className="relative flex-1">
        <span className="block text-lg font-extrabold">{title}</span>
        <span className={cx('block text-[13px]', active ? 'text-white/85' : 'text-muted')}>{sub}</span>
      </span>
      {active && <Check className="relative h-6 w-6" />}
    </button>
  )

  const hasFriends = value.types.includes('friends')
  const hasRel = value.types.includes('relationship')

  return (
    <div className="space-y-3">
      {types.map((t) =>
        card(
          t.slug, t.emoji, t.name,
          t.slug === 'friends' ? 'Hang out, play, study, travel…' : t.slug === 'relationship' ? 'Dating & something meaningful' : 'Open to this',
          value.types.length === 1 && value.types[0] === t.slug,
          t.slug === 'friends' ? 'friend-gradient' : t.slug === 'relationship' ? 'love-gradient' : 'brand-gradient',
          () => set({ types: [t.slug] }),
        ),
      )}
      {types.length > 1 && card('both', '✨', 'Both', 'Friends and romance — see everyone', allSelected, 'both-gradient', () => set({ types: types.map((t) => t.slug) }))}

      {hasFriends && (
        <div className="animate-rise pt-3">
          <p className="mb-2 px-1 text-[13px] font-semibold text-muted">What kind of friends? <span className="font-normal">(optional)</span></p>
          <ChipGroup
            multi tone="friend"
            options={catalog.friendship_kinds.map((k) => ({ v: k.slug, label: k.name, emoji: k.emoji }))}
            value={value.kinds}
            onChange={(v) => set({ kinds: value.kinds.includes(v) ? value.kinds.filter((x) => x !== v) : [...value.kinds, v] })}
          />
        </div>
      )}

      {hasRel && (
        <div className="animate-rise space-y-4 pt-3">
          <div>
            <p className="mb-2 px-1 text-[13px] font-semibold text-muted">What kind of relationship?</p>
            <ChipGroup tone="love" options={INTENTIONS} value={value.intention} onChange={(v) => set({ intention: v })} />
          </div>
          <div>
            <p className="mb-2 px-1 text-[13px] font-semibold text-muted">For dating, show me</p>
            <ChipGroup
              multi tone="love" options={PREF_GENDERS} value={value.genders}
              onChange={(v) => {
                if (v === 'everyone') return set({ genders: ['everyone'] })
                const base = value.genders.filter((g) => g !== 'everyone')
                const next = base.includes(v) ? base.filter((g) => g !== v) : [...base, v]
                set({ genders: next.length ? next : ['everyone'] })
              }}
            />
          </div>
        </div>
      )}

      {value.types.length > 0 && (
        <div className="animate-rise space-y-5 rounded-xl2 border border-line/70 bg-surface p-4 shadow-soft">
          <AgeRange min={value.ageMin} max={value.ageMax} floor={minAge} onChange={(a, b) => set({ ageMin: a, ageMax: b })} />
          <div>
            <p className="mb-2 text-[13px] font-semibold text-muted">How far should we look?</p>
            <ChipGroup options={SCOPES} value={value.scope} onChange={(v) => set({ scope: v })} />
          </div>
        </div>
      )}
    </div>
  )
}

/** Persists an IntentState using the two profile endpoints. */
export async function saveIntent(v: IntentState): Promise<Me> {
  await api('PATCH', '/api/me/profile', { body: { connection_types: v.types } })
  return api<Me>('PATCH', '/api/me/preferences', {
    body: {
      friendship_kinds: v.types.includes('friends') ? v.kinds : [],
      preferences: {
        relationship_intention: v.types.includes('relationship') ? v.intention : '',
        preferred_genders: v.genders.length ? v.genders : ['everyone'],
        age_min: v.ageMin,
        age_max: v.ageMax,
        distance_scope: v.scope,
      },
    },
  })
}

/* ───────────────── Interests ───────────────── */

export function InterestPicker({ value, onChange, catalog, max = 15 }: { value: string[]; onChange: (v: string[]) => void; catalog: Catalog; max?: number }) {
  const groups = useMemo(() => {
    const m = new Map<string, typeof catalog.interests>()
    for (const i of catalog.interests) m.set(i.category, [...(m.get(i.category) ?? []), i])
    return [...m.entries()]
  }, [catalog])
  const toggle = (s: string) => {
    if (value.includes(s)) return onChange(value.filter((x) => x !== s))
    if (value.length >= max) return toastError(new Error(`You can pick up to ${max}`))
    onChange([...value, s])
  }
  return (
    <div className="space-y-5">
      {groups.map(([cat, items]) => (
        <div key={cat}>
          <p className="mb-2 px-1 text-[13px] font-bold uppercase tracking-wide text-muted">{cat}</p>
          <div className="flex flex-wrap gap-2">
            {items.map((i) => (
              <Chip key={i.slug} selected={value.includes(i.slug)} onClick={() => toggle(i.slug)}>
                <span>{i.emoji}</span>
                {i.name}
              </Chip>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

/* ───────────────── Languages ───────────────── */

export function LanguagePicker({ value, onChange, catalog, detailed }: { value: UserLanguage[]; onChange: (v: UserLanguage[]) => void; catalog: Catalog; detailed?: boolean }) {
  const [q, setQ] = useState('')
  const list = catalog.languages.filter((l) => !q || l.name.toLowerCase().includes(q.toLowerCase()) || l.native_name.toLowerCase().includes(q.toLowerCase()))
  const has = (c: string) => value.find((l) => l.code === c)
  const toggle = (c: string) => {
    if (has(c)) return onChange(value.filter((l) => l.code !== c))
    if (value.length >= 8) return toastError(new Error('Up to 8 languages'))
    onChange([...value, { code: c, level: 'intermediate', wants_practice: false }])
  }
  const upd = (c: string, p: Partial<UserLanguage>) => onChange(value.map((l) => (l.code === c ? { ...l, ...p } : l)))
  return (
    <div>
      {detailed && value.length > 0 && (
        <div className="mb-5 space-y-3">
          {value.map((l) => {
            const info = catalog.languages.find((x) => x.code === l.code)
            return (
              <div key={l.code} className="rounded-2xl border border-line/70 bg-surface p-3.5 shadow-soft">
                <div className="mb-2.5 flex items-center justify-between">
                  <span className="font-bold">{info?.name ?? l.code} <span className="text-xs font-normal text-muted">{info?.native_name}</span></span>
                  <button onClick={() => toggle(l.code)} className="press text-faint" aria-label="Remove"><X className="h-5 w-5" /></button>
                </div>
                <ChipGroup options={LEVELS} value={l.level} onChange={(v) => upd(l.code, { level: v as UserLanguage['level'] })} />
                <label className="mt-3 flex items-center gap-2 text-sm text-muted">
                  <input type="checkbox" checked={l.wants_practice} onChange={(e) => upd(l.code, { wants_practice: e.target.checked })} className="h-4 w-4 accent-[rgb(var(--brand))]" />
                  I want to practice this language
                </label>
              </div>
            )
          })}
        </div>
      )}
      <div className="relative mb-3">
        <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
        <input className="field !pl-10" placeholder="Search languages" value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      <div className="flex flex-wrap gap-2">
        {list.slice(0, q ? 60 : 24).map((l) => (
          <Chip key={l.code} selected={!!has(l.code)} onClick={() => toggle(l.code)}>
            {l.name}
          </Chip>
        ))}
      </div>
    </div>
  )
}

/* ───────────────── Location ───────────────── */

export function LocationPicker({ value, onChange }: { value: Loc | null; onChange: (l: Loc) => void }) {
  const initialCountry = value?.country_code || (navigator.language.split('-')[1] ?? 'IT').toUpperCase()
  const [country, setCountry] = useState(COUNTRY_CODES.includes(initialCountry) ? initialCountry : 'US')
  const [cityQ, setCityQ] = useState(value?.city ?? '')
  const [city, setCity] = useState<Loc | null>(value && !value.area ? value : value ? { ...value, area: '' } : null)
  const [cities, setCities] = useState<Loc[]>([])
  const [areas, setAreas] = useState<Loc[]>([])
  const [area, setArea] = useState(value?.area ?? '')
  const [newArea, setNewArea] = useState('')
  const [busy, setBusy] = useState(false)
  const dq = useDebounced(cityQ, 250)

  const countries = useMemo(() => COUNTRY_CODES.map((c) => ({ c, n: countryName(c) })).sort((a, b) => a.n.localeCompare(b.n)), [])

  useEffect(() => {
    if (city && dq.trim().toLowerCase() === city.city.toLowerCase()) return
    let live = true
    get<{ items: Loc[] }>('/api/locations/cities', { country, q: dq.trim() }).then((r) => live && setCities(r.items)).catch(() => {})
    return () => { live = false }
  }, [country, dq, city])

  useEffect(() => {
    if (!city) return setAreas([])
    let live = true
    get<{ items: Loc[] }>('/api/locations/areas', { country: city.country_code, city: city.city }).then((r) => live && setAreas(r.items)).catch(() => {})
    return () => { live = false }
  }, [city])

  const chooseCity = (l: Loc) => {
    setCity(l); setCityQ(l.city); setArea(''); setCities([])
    onChange(l)
    platform.haptic('select')
  }

  const createCity = async () => {
    const name = cityQ.trim()
    if (name.length < 2) return
    setBusy(true)
    try {
      chooseCity(await post<Loc>('/api/locations', { country_code: country, country: countryName(country), city: name }))
    } catch (e) { toastError(e) } finally { setBusy(false) }
  }

  const chooseArea = (l: Loc | null) => {
    if (!city) return
    if (!l) { setArea(''); onChange(city); return }
    setArea(l.area); onChange(l)
    platform.haptic('select')
  }

  const addArea = async () => {
    if (!city || newArea.trim().length < 2) return
    setBusy(true)
    try {
      const l = await post<Loc>('/api/locations', { country_code: city.country_code, country: city.country, region: city.region, city: city.city, area: newArea.trim() })
      setAreas((a) => [...a.filter((x) => x.id !== l.id), l]); setNewArea(''); chooseArea(l)
    } catch (e) { toastError(e) } finally { setBusy(false) }
  }

  const exact = cities.some((c) => c.city.toLowerCase() === cityQ.trim().toLowerCase())

  return (
    <div className="space-y-4">
      <div>
        <span className="mb-1.5 block px-1 text-[13px] font-semibold text-muted">Country</span>
        <select
          className="field appearance-none"
          value={country}
          onChange={(e) => { setCountry(e.target.value); setCity(null); setCityQ(''); setAreas([]) }}
        >
          {countries.map((c) => <option key={c.c} value={c.c}>{flag(c.c)}  {c.n}</option>)}
        </select>
      </div>

      <div>
        <span className="mb-1.5 block px-1 text-[13px] font-semibold text-muted">City</span>
        <div className="relative">
          <MapPin className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
          <input
            className="field !pl-10"
            placeholder="Start typing your city"
            value={cityQ}
            onChange={(e) => { setCityQ(e.target.value); if (city) setCity(null) }}
            autoComplete="off"
          />
        </div>
        {!city && (
          <div className="mt-2 overflow-hidden rounded-2xl border border-line/70 bg-surface shadow-soft">
            {cities.map((c) => (
              <button key={c.id} onClick={() => chooseCity(c)} className="press flex w-full items-center justify-between border-b border-line/60 px-4 py-3 text-left last:border-0 active:bg-elevated">
                <span className="font-semibold">{c.city}</span>
                <span className="text-xs text-muted">{c.region}</span>
              </button>
            ))}
            {cityQ.trim().length >= 2 && !exact && (
              <button onClick={createCity} disabled={busy} className="press flex w-full items-center gap-2 px-4 py-3 text-left font-semibold text-brand">
                <Plus className="h-4 w-4" /> Use “{cityQ.trim()}”
              </button>
            )}
            {cities.length === 0 && cityQ.trim().length < 2 && <p className="px-4 py-3 text-sm text-muted">Popular cities appear here as you type</p>}
          </div>
        )}
      </div>

      {city && (
        <div className="animate-rise">
          <span className="mb-1.5 block px-1 text-[13px] font-semibold text-muted">
            Area <span className="font-normal">(optional — never an exact address)</span>
          </span>
          <div className="mb-2 flex flex-wrap gap-2">
            <Chip selected={!area} onClick={() => chooseArea(null)}>Whole city</Chip>
            {areas.map((a) => (
              <Chip key={a.id} selected={area.toLowerCase() === a.area.toLowerCase()} onClick={() => chooseArea(a)}>{a.area}</Chip>
            ))}
          </div>
          <div className="flex gap-2">
            <input className="field" placeholder="Add your neighborhood" value={newArea} onChange={(e) => setNewArea(e.target.value)} maxLength={40} />
            <Button variant="soft" onClick={addArea} loading={busy} disabled={newArea.trim().length < 2}>Add</Button>
          </div>
        </div>
      )}
    </div>
  )
}

/* ───────────────── Username ───────────────── */

export function UsernameField({ initial, onValid }: { initial: string; onValid: (suffix: string | null) => void }) {
  const [v, setV] = useState(initial)
  const [state, setState] = useState<'idle' | 'checking' | 'ok' | 'bad'>('idle')
  const [msg, setMsg] = useState('')
  const dv = useDebounced(v, 400)
  const cb = useRef(onValid)
  cb.current = onValid

  useEffect(() => {
    if (!dv) { setState('idle'); setMsg(''); cb.current(null); return }
    let live = true
    setState('checking')
    get('/api/username/check', { suffix: dv })
      .then(() => { if (live) { setState('ok'); setMsg(''); cb.current(dv) } })
      .catch((e: Error) => { if (live) { setState('bad'); setMsg(e.message); cb.current(null) } })
    return () => { live = false }
  }, [dv])

  return (
    <div>
      <div className="relative">
        <div className="field flex items-center !py-0 focus-within:!border-brand">
          <span className="select-none text-[15px] font-bold text-gradient">Atish_</span>
          <input
            className="h-[52px] min-w-0 flex-1 bg-transparent pl-0.5 text-[15px] font-semibold outline-none placeholder:font-normal placeholder:text-faint"
            placeholder="your_name"
            value={v}
            maxLength={24}
            autoCapitalize="off"
            autoCorrect="off"
            spellCheck={false}
            onChange={(e) => setV(e.target.value.replace(/[^A-Za-z0-9_]/g, ''))}
          />
          {state === 'checking' && <Spinner className="h-5 w-5" />}
          {state === 'ok' && <Check className="h-5 w-5 text-ok" />}
          {state === 'bad' && <AlertCircle className="h-5 w-5 text-danger" />}
        </div>
      </div>
      <p className={cx('mt-1.5 px-1 text-xs', state === 'bad' ? 'text-danger' : 'text-muted')}>
        {state === 'bad' ? msg : state === 'ok' ? `You'll be ${'Atish_' + v}` : 'This is your public Atish username. 3–24 letters, numbers or _'}
      </p>
    </div>
  )
}


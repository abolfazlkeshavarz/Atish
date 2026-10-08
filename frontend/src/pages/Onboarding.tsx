import { Logo } from '@/components/Logo'
import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft, Pencil } from 'lucide-react'
import { patch, post, put } from '@/api/client'
import type { Me, UserLanguage } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError } from '@/store/ui'
import { platform } from '@/platform/telegram'
import { Button, ChipGroup, Field, Progress, cx } from '@/components/ui'
import {
  IntentEditor, InterestPicker, LanguagePicker, LocationPicker, PhotoManager, UsernameField, intentFromMe, saveIntent, type IntentState,
} from '@/components/editors'
import { GENDERS } from '@/lib/format'

const STEPS = [
  { key: 'you', title: 'Tell us about you', sub: 'The basics — takes 20 seconds' },
  { key: 'photo', title: 'Add your photo', sub: 'Profiles with a smile get the most matches' },
  { key: 'intent', title: 'What are you looking for?', sub: 'We only show you people who want the same' },
  { key: 'where', title: 'Where are you?', sub: 'City & area only — never your exact location' },
  { key: 'interests', title: 'What are you into?', sub: 'Pick at least 3 — more means better matches' },
  { key: 'extras', title: 'Final touches', sub: 'Optional — you can skip this' },
] as const

function startStep(me: Me): number {
  const m = me.onboarding.missing
  if (m.includes('username') || m.includes('basics')) return 0
  if (m.includes('photo')) return 1
  if (m.includes('connection_types')) return 2
  if (m.includes('location')) return 3
  if (m.includes('interests')) return 4
  return 5
}

export default function Onboarding() {
  const nav = useNavigate()
  const { me, catalog, app, setMe } = useSession()
  const [step, setStep] = useState(() => (me ? startStep(me) : 0))
  const [busy, setBusy] = useState(false)
  const minAge = app?.min_age ?? 18

  // step 1
  const tgUser = platform.prefill()
  const [name, setName] = useState(me?.display_name || tgUser?.first_name || '')
  const [birth, setBirth] = useState(me?.birth_date || '')
  const [gender, setGender] = useState(me?.gender || '')
  const [editingName, setEditingName] = useState(!me?.atish_username)
  const [suffix, setSuffix] = useState<string | null>(me?.atish_username ? me.atish_username.replace('Atish_', '') : null)

  // step 3
  const [intent, setIntent] = useState<IntentState>(() => {
    const base = me ? intentFromMe(me) : ({ types: [], kinds: [], intention: '', genders: ['everyone'], ageMin: 18, ageMax: 99, scope: 'same_city' } as IntentState)
    if (me && base.ageMin === 18 && base.ageMax === 99 && me.age) {
      return { ...base, ageMin: Math.max(minAge, me.age - 6), ageMax: Math.min(99, me.age + 10) }
    }
    return base
  })
  // step 4
  const [locId, setLocId] = useState<number | null>(me?.location?.id ?? null)
  // step 5
  const [interests, setInterests] = useState<string[]>(me?.interests ?? [])
  // step 6
  const [bio, setBio] = useState(me?.bio ?? '')
  const [langs, setLangs] = useState<UserLanguage[]>(me?.languages ?? [])

  const maxBirth = useMemo(() => {
    const d = new Date()
    d.setFullYear(d.getFullYear() - minAge)
    return d.toISOString().slice(0, 10)
  }, [minAge])

  if (!me || !catalog) return null
  const cur = STEPS[step]

  const canContinue = (() => {
    switch (cur.key) {
      case 'you': return name.trim().length >= 2 && !!birth && birth <= maxBirth && !!gender && (!!suffix || (!editingName && !!me.atish_username))
      case 'photo': return me.photos.length > 0
      case 'intent': return intent.types.length > 0
      case 'where': return locId !== null
      case 'interests': return interests.length >= 3
      default: return true
    }
  })()

  const next = async () => {
    setBusy(true)
    try {
      switch (cur.key) {
        case 'you': {
          if (suffix && 'Atish_' + suffix !== me.atish_username) await put('/api/me/username', { suffix })
          setMe(await patch<Me>('/api/me/profile', { display_name: name.trim(), birth_date: birth, gender }))
          break
        }
        case 'intent':
          setMe(await saveIntent({ ...intent, intention: intent.types.includes('relationship') ? intent.intention || 'not_sure' : '' }))
          break
        case 'where':
          setMe(await patch<Me>('/api/me/profile', { location_id: locId }))
          break
        case 'interests':
          setMe(await patch<Me>('/api/me/profile', { interests }))
          break
        case 'extras':
          await patch('/api/me/profile', { bio: bio.trim(), languages: langs })
          setMe(await post<Me>('/api/me/onboarding/complete'))
          platform.haptic('success')
          toast("You're in! Let's find your people 🔥", 'success')
          nav('/', { replace: true })
          return
      }
      platform.haptic('light')
      setStep((s) => s + 1)
    } catch (e) {
      toastError(e)
    } finally {
      setBusy(false)
    }
  }

  const finishLater = async () => {
    setBusy(true)
    try {
      setMe(await post<Me>('/api/me/onboarding/complete'))
      nav('/', { replace: true })
    } catch (e) { toastError(e) } finally { setBusy(false) }
  }

  return (
    <div className="mx-auto flex h-full max-w-xl flex-col">
      <header className="px-5 pb-3" style={{ paddingTop: 'calc(14px + var(--safe-t))' }}>
        <div className="mb-4 flex items-center justify-between">
          <button
            onClick={() => setStep((s) => Math.max(0, s - 1))}
            className={cx('press flex h-10 w-10 items-center justify-center rounded-full bg-elevated', step === 0 && 'invisible')}
            aria-label="Back"
          >
            <ArrowLeft className="h-5 w-5" />
          </button>
          <div className="flex items-center gap-1.5 font-extrabold">
            <Logo tile size={28} />
            <span className="text-gradient text-lg">Atish</span>
          </div>
          <span className="w-10 text-right text-xs font-semibold text-muted">{step + 1}/{STEPS.length}</span>
        </div>
        <Progress value={((step + 1) / STEPS.length) * 100} />
        {(me?.role === 'admin' || me?.role === 'moderator') && (
          <button onClick={() => nav('/admin')} className="press mt-3 w-full rounded-xl bg-elevated py-2 text-sm font-semibold text-muted">
            Staff: open the admin panel
          </button>
        )}
      </header>

      <main className="scroll-hide flex-1 overflow-y-auto px-5 pb-6">
        <div key={cur.key} className="animate-rise">
          <h1 className="mt-3 text-[28px] font-extrabold leading-tight">{cur.title}</h1>
          <p className="mb-6 mt-1 text-[15px] text-muted">{cur.sub}</p>

          {cur.key === 'you' && (
            <div>
              <Field label="First name">
                <input className="field" value={name} maxLength={40} onChange={(e) => setName(e.target.value)} placeholder="How should we call you?" autoComplete="given-name" />
              </Field>
              <Field label="Date of birth" hint={`You must be ${minAge}+. Only your age is shown to others.`}>
                <input className="field" type="date" value={birth} max={maxBirth} min="1920-01-01" onChange={(e) => setBirth(e.target.value)} />
              </Field>
              <Field label="I am">
                <ChipGroup options={GENDERS} value={gender} onChange={setGender} />
              </Field>
              <Field label="Your Atish username">
                {editingName ? (
                  <UsernameField initial={suffix ?? ''} onValid={setSuffix} />
                ) : (
                  <button onClick={() => setEditingName(true)} className="press field flex items-center justify-between !py-3">
                    <span className="font-bold"><span className="text-gradient">Atish_</span>{me.atish_username?.replace('Atish_', '')}</span>
                    <span className="flex items-center gap-1 text-sm font-semibold text-brand"><Pencil className="h-4 w-4" /> Edit</span>
                  </button>
                )}
              </Field>
            </div>
          )}

          {cur.key === 'photo' && (
            <div className="pt-2">
              <PhotoManager hero />
              <p className="mt-6 text-center text-xs text-muted">Clear face photos work best. You can add more later.</p>
            </div>
          )}

          {cur.key === 'intent' && <IntentEditor value={intent} onChange={setIntent} catalog={catalog} minAge={minAge} />}

          {cur.key === 'where' && <LocationPicker value={me.location} onChange={(l) => setLocId(l.id)} />}

          {cur.key === 'interests' && (
            <>
              <div className="sticky top-0 z-10 -mx-5 mb-3 bg-bg/90 px-5 py-2 backdrop-blur">
                <span className={cx('rounded-full px-3 py-1 text-sm font-bold', interests.length >= 3 ? 'bg-ok/15 text-ok' : 'bg-elevated text-muted')}>
                  {interests.length} selected{interests.length < 3 ? ` · ${3 - interests.length} more` : interests.length < 5 ? ' · 5+ is ideal' : ''}
                </span>
              </div>
              <InterestPicker value={interests} onChange={setInterests} catalog={catalog} />
            </>
          )}

          {cur.key === 'extras' && (
            <div>
              <Field label="A line about you" hint={`${bio.length}/300`}>
                <textarea
                  className="field min-h-[110px] resize-none"
                  maxLength={300}
                  value={bio}
                  onChange={(e) => setBio(e.target.value)}
                  placeholder="CS student into space, gaming and exploring new places…"
                />
              </Field>
              <p className="mb-2 px-1 text-[13px] font-semibold text-muted">Languages you speak</p>
              <LanguagePicker value={langs} onChange={setLangs} catalog={catalog} />
            </div>
          )}
        </div>
      </main>

      <footer className="border-t border-line/60 bg-bg/90 px-5 pt-3 backdrop-blur" style={{ paddingBottom: 'calc(14px + var(--safe-b))' }}>
        <Button size="lg" block onClick={next} disabled={!canContinue} loading={busy}>
          {cur.key === 'extras' ? 'Start discovering 🔥' : 'Continue'}
        </Button>
        {cur.key === 'extras' && (
          <button onClick={finishLater} disabled={busy} className="press mt-2 w-full py-2 text-sm font-semibold text-muted">
            Skip for now
          </button>
        )}
      </footer>
    </div>
  )
}


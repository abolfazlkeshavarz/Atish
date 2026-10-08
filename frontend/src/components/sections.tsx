import { useState, type ReactNode } from 'react'
import { Briefcase, Camera, CalendarClock, Heart, Languages, MapPin, Sparkles, User, Wine } from 'lucide-react'
import { patch } from '@/api/client'
import type { Me, Occupation, Lifestyle, Slot, UserLanguage } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError } from '@/store/ui'
import { Button, ChipGroup, Field, Sheet, cx } from '@/components/ui'
import {
  IntentEditor, InterestPicker, LanguagePicker, LocationPicker, PhotoManager, intentFromMe, saveIntent, type IntentState,
} from '@/components/editors'
import { DAYS, GENDERS, LIFESTYLE, OCCUPATION, SLOTS, flag } from '@/lib/format'

export type SectionKey = 'photos' | 'about' | 'intent' | 'interests' | 'languages' | 'location' | 'work' | 'lifestyle' | 'availability'

export const SECTIONS: { key: SectionKey; title: string; icon: ReactNode; summary: (m: Me) => string }[] = [
  { key: 'photos', title: 'Photos', icon: <Camera className="h-5 w-5" />, summary: (m) => `${m.photos.length} photo${m.photos.length === 1 ? '' : 's'}` },
  { key: 'about', title: 'About me', icon: <User className="h-5 w-5" />, summary: (m) => m.bio || 'Add a short bio' },
  { key: 'intent', title: 'Looking for', icon: <Heart className="h-5 w-5" />, summary: (m) => m.connection_types.map((t) => (t === 'friends' ? '🤝 Friends' : '❤️ Relationship')).join(' · ') },
  { key: 'interests', title: 'Interests', icon: <Sparkles className="h-5 w-5" />, summary: (m) => `${m.interests.length} selected` },
  { key: 'languages', title: 'Languages', icon: <Languages className="h-5 w-5" />, summary: (m) => (m.languages.length ? m.languages.map((l) => l.code.toUpperCase()).join(', ') : 'Add languages') },
  { key: 'location', title: 'Location', icon: <MapPin className="h-5 w-5" />, summary: (m) => (m.location ? `${flag(m.location.country_code)} ${m.location.city}${m.location.area ? ' · ' + m.location.area : ''}` : 'Set your area') },
  { key: 'work', title: 'Work & education', icon: <Briefcase className="h-5 w-5" />, summary: (m) => m.occupation.profession || m.occupation.university || 'Optional' },
  { key: 'lifestyle', title: 'Lifestyle', icon: <Wine className="h-5 w-5" />, summary: () => 'Optional' },
  { key: 'availability', title: 'When I\'m free', icon: <CalendarClock className="h-5 w-5" />, summary: (m) => (m.availability.length ? `${m.availability.length} time slots` : 'Optional') },
]

export function SectionSheet({ section, onClose }: { section: SectionKey | null; onClose: () => void }) {
  const { me, catalog, app, setMe } = useSession()
  if (!section || !me || !catalog) return null
  const meta = SECTIONS.find((s) => s.key === section)!
  return (
    <Body key={section} section={section} title={meta.title} me={me} catalogReady onClose={onClose} setMe={setMe} minAge={app?.min_age ?? 18} />
  )
}

function Body({ section, title, me, onClose, setMe, minAge }: {
  section: SectionKey; title: string; me: Me; catalogReady: boolean; onClose: () => void; setMe: (m: Me) => void; minAge: number
}) {
  const catalog = useSession((s) => s.catalog)!
  const [busy, setBusy] = useState(false)

  // local drafts
  const [name, setName] = useState(me.display_name)
  const [bio, setBio] = useState(me.bio)
  const [gender, setGender] = useState(me.gender)
  const [intent, setIntent] = useState<IntentState>(intentFromMe(me))
  const [interests, setInterests] = useState(me.interests)
  const [langs, setLangs] = useState<UserLanguage[]>(me.languages)
  const [locId, setLocId] = useState<number | null>(me.location?.id ?? null)
  const [occ, setOcc] = useState<Occupation>(me.occupation)
  const [life, setLife] = useState<Lifestyle>(me.lifestyle)
  const [slots, setSlots] = useState<Slot[]>(me.availability)

  const save = async () => {
    setBusy(true)
    try {
      let next: Me
      switch (section) {
        case 'about': next = await patch<Me>('/api/me/profile', { display_name: name.trim(), bio: bio.trim(), gender }); break
        case 'intent': next = await saveIntent({ ...intent, intention: intent.types.includes('relationship') ? intent.intention || 'not_sure' : '' }); break
        case 'interests': next = await patch<Me>('/api/me/profile', { interests }); break
        case 'languages': next = await patch<Me>('/api/me/profile', { languages: langs }); break
        case 'location': next = await patch<Me>('/api/me/profile', { location_id: locId }); break
        case 'work': next = await patch<Me>('/api/me/profile', { occupation: occ }); break
        case 'lifestyle': next = await patch<Me>('/api/me/profile', { lifestyle: life }); break
        case 'availability': next = await patch<Me>('/api/me/preferences', { availability: slots }); break
        default: return onClose()
      }
      setMe(next)
      toast('Saved', 'success')
      onClose()
    } catch (e) {
      toastError(e)
    } finally {
      setBusy(false)
    }
  }

  const valid =
    section === 'about' ? name.trim().length >= 2
    : section === 'intent' ? intent.types.length > 0
    : section === 'interests' ? interests.length >= 3
    : section === 'location' ? locId !== null
    : true

  const toggleSlot = (day: number, slot: string) =>
    setSlots((s) => (s.some((x) => x.day === day && x.slot === slot) ? s.filter((x) => !(x.day === day && x.slot === slot)) : [...s, { day, slot }]))

  return (
    <Sheet
      open
      onClose={onClose}
      title={title}
      tall={section !== 'about' && section !== 'work' && section !== 'lifestyle'}
      footer={section === 'photos' ? <Button block onClick={onClose}>Done</Button> : <Button block loading={busy} disabled={!valid} onClick={save}>Save</Button>}
    >
      {section === 'photos' && (
        <>
          <PhotoManager />
          <p className="mt-4 text-center text-xs text-muted">Your first photo is your main one. Photos are re-processed to remove hidden location data.</p>
        </>
      )}
      {section === 'about' && (
        <>
          <Field label="Name"><input className="field" value={name} maxLength={40} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="Bio" hint={`${bio.length}/300`}>
            <textarea className="field min-h-[120px] resize-none" maxLength={300} value={bio} onChange={(e) => setBio(e.target.value)} placeholder="Tell people what you're about…" />
          </Field>
          <Field label="Gender"><ChipGroup options={GENDERS} value={gender} onChange={setGender} /></Field>
        </>
      )}
      {section === 'intent' && <IntentEditor value={intent} onChange={setIntent} catalog={catalog} minAge={minAge} />}
      {section === 'interests' && (
        <>
          <p className="mb-3 text-sm text-muted">{interests.length} selected · pick 5–15 for the best matches</p>
          <InterestPicker value={interests} onChange={setInterests} catalog={catalog} />
        </>
      )}
      {section === 'languages' && <LanguagePicker value={langs} onChange={setLangs} catalog={catalog} detailed />}
      {section === 'location' && <LocationPicker value={me.location} onChange={(l) => setLocId(l.id)} />}
      {section === 'work' && (
        <>
          <Field label="I am"><ChipGroup options={OCCUPATION} value={occ.occupation_status} onChange={(v) => setOcc({ ...occ, occupation_status: occ.occupation_status === v ? '' : v })} /></Field>
          {(['university', 'field_of_study', 'degree', 'profession', 'industry'] as const).map((k) => (
            <Field key={k} label={k.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase())}>
              <input className="field" maxLength={80} value={occ[k]} onChange={(e) => setOcc({ ...occ, [k]: e.target.value })} />
            </Field>
          ))}
        </>
      )}
      {section === 'lifestyle' && (
        <>
          {LIFESTYLE.map((l) => (
            <Field key={l.key} label={l.label}>
              <ChipGroup
                options={l.options}
                value={(life as unknown as Record<string, string>)[l.key]}
                onChange={(v) => setLife({ ...life, [l.key]: (life as unknown as Record<string, string>)[l.key] === v ? '' : v })}
              />
            </Field>
          ))}
        </>
      )}
      {section === 'availability' && (
        <div>
          <p className="mb-4 text-sm text-muted">When are you usually free? We'll show what you have in common.</p>
          <div className="grid grid-cols-[44px_repeat(4,1fr)] gap-1.5 text-center text-xs">
            <span />
            {SLOTS.map((s) => <span key={s.v} className="pb-1 font-semibold text-muted">{s.emoji}<br />{s.label}</span>)}
            {DAYS.map((d, di) => (
              <div key={d} className="contents">
                <span className="flex items-center text-sm font-bold">{d}</span>
                {SLOTS.map((s) => {
                  const on = slots.some((x) => x.day === di && x.slot === s.v)
                  return (
                    <button key={s.v} onClick={() => toggleSlot(di, s.v)} aria-pressed={on}
                      className={cx('press h-10 rounded-xl border transition', on ? 'brand-gradient border-transparent text-white shadow-soft' : 'border-line bg-surface')}>
                      {on && '✓'}
                    </button>
                  )
                })}
              </div>
            ))}
          </div>
        </div>
      )}
    </Sheet>
  )
}


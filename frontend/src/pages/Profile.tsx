import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ChevronRight, Crown, Pencil, Settings as Cog } from 'lucide-react'
import { VerifyPill } from '@/components/VerifyBadge'
import { useSession } from '@/store/session'
import { useUI } from '@/store/ui'
import { Card, Group, Progress, Row, cx } from '@/components/ui'
import { IntentBadges } from '@/components/profile'
import { SECTIONS, SectionSheet, type SectionKey } from '@/components/sections'
import { flag } from '@/lib/format'

const SUGGESTIONS: Record<string, { label: string; sub: string; emoji: string; to?: string; section?: SectionKey }> = {
  photo: { label: 'Add a photo', sub: 'Profiles with photos get 10× more likes', emoji: '📸', section: 'photos' },
  more_photos: { label: 'Add more photos', sub: 'Show a different side of you', emoji: '🖼️', section: 'photos' },
  bio: { label: 'Write a short bio', sub: 'A line or two goes a long way', emoji: '✍️', section: 'about' },
  interests: { label: 'Pick more interests', sub: '5+ interests = better matches', emoji: '🎯', section: 'interests' },
  languages: { label: 'Add your languages', sub: 'Find language-exchange partners', emoji: '🗣️', section: 'languages' },
  preferences: { label: 'Set your preferences', sub: 'What kind of friends or dates?', emoji: '💞', section: 'intent' },
  personality: { label: 'Take the 30-second quiz', sub: 'Smarter matches, no long test', emoji: '🧩', to: '/quiz' },
  availability: { label: 'Say when you\'re free', sub: 'Match on weekend plans', emoji: '🗓️', section: 'availability' },
  work: { label: 'Add work or school', sub: 'Optional', emoji: '🎓', section: 'work' },
  phone: { label: 'Verify your phone', sub: 'Get a trust badge — never shown publicly', emoji: '✅', to: '/settings' },
}

export default function Profile() {
  const nav = useNavigate()
  const me = useSession((s) => s.me)!
  const openPremium = useUI((s) => s.openPremium)
  const [section, setSection] = useState<SectionKey | null>(null)
  const premiumOn = me.app.premium_enabled

  return (
    <div className="px-4 pb-8" style={{ paddingTop: 'calc(14px + var(--safe-t))' }}>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-[28px] font-extrabold">Profile</h1>
        <button onClick={() => nav('/settings')} className="press flex h-11 w-11 items-center justify-center rounded-full bg-surface shadow-soft" aria-label="Settings">
          <Cog className="h-5 w-5" />
        </button>
      </div>

      <Card className="mb-5 overflow-hidden">
        <button onClick={() => setSection('photos')} className="press relative block aspect-[4/3] w-full bg-elevated">
          {me.photos[0] ? <img src={me.photos[0].url} alt="" className="h-full w-full object-cover" /> : <div className="ph-bg h-full w-full" />}
          <div className="absolute inset-0 bg-gradient-to-t from-black/70 via-transparent to-transparent" />
          <span className="absolute right-3 top-3 flex h-9 w-9 items-center justify-center rounded-full bg-white/25 text-white backdrop-blur"><Pencil className="h-4 w-4" /></span>
          <div className="absolute inset-x-0 bottom-0 p-4 text-left text-white">
            <div className="text-[26px] font-extrabold leading-tight">
              {me.display_name}, {me.age}
              {me.entitlements.premium && premiumOn && <Crown className="ml-1.5 inline h-5 w-5 text-amber-300" />}
            </div>
            <div className="text-sm text-white/85">{me.atish_username} · {me.location && `${flag(me.location.country_code)} ${me.location.city}`}</div>
          </div>
        </button>
        <div className="flex flex-wrap items-center gap-2 p-4">
          <IntentBadges types={me.connection_types} />
          {me.telegram_verified && <VerifyPill kind="telegram" label="Telegram" />}
          {me.phone_verified ? <VerifyPill kind="phone" label="Phone" /> : <VerifyPill kind="phone" label="Verify phone" muted onClick={() => nav('/settings')} />}
          {me.photo_verified && <VerifyPill kind="photo" label="Photo" />}
          {me.identity_verified && <VerifyPill kind="identity" label="ID" />}
        </div>
      </Card>

      {me.completeness < 100 && (
        <Card className="mb-5 p-4">
          <div className="mb-2 flex items-end justify-between">
            <div>
              <div className="text-[15px] font-extrabold">Profile strength</div>
              <div className="text-xs text-muted">Complete profiles find better matches</div>
            </div>
            <div className="text-gradient text-2xl font-black">{me.completeness}%</div>
          </div>
          <Progress value={me.completeness} />
          <div className="mt-3 space-y-1.5">
            {me.suggestions.slice(0, 3).map((k) => {
              const s = SUGGESTIONS[k]
              if (!s) return null
              return (
                <button key={k} onClick={() => (s.to ? nav(s.to) : setSection(s.section!))} className="press flex w-full items-center gap-3 rounded-2xl bg-elevated/70 px-3 py-2.5 text-left">
                  <span className="text-xl">{s.emoji}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block text-sm font-bold">{s.label}</span>
                    <span className="block truncate text-xs text-muted">{s.sub}</span>
                  </span>
                  <ChevronRight className="h-4 w-4 text-faint" />
                </button>
              )
            })}
          </div>
        </Card>
      )}

      {premiumOn && !me.entitlements.premium && (
        <button onClick={() => openPremium('profile')} className="press brand-gradient mb-5 flex w-full items-center gap-4 rounded-xl2 p-4 text-left text-white shadow-glow">
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/20"><Crown className="h-6 w-6" /></span>
          <span className="flex-1">
            <span className="block text-lg font-extrabold">Atish Plus</span>
            <span className="block text-sm text-white/85">Unlimited likes · See who likes you · Rewind</span>
          </span>
          <ChevronRight className="h-5 w-5" />
        </button>
      )}

      <h2 className="mb-2 px-1 text-[15px] font-bold">Edit profile</h2>
      <Group>
        {SECTIONS.map((s) => (
          <Row key={s.key} icon={s.icon} title={s.title} sub={s.summary(me)} onClick={() => setSection(s.key)} right={<ChevronRight className={cx('h-4 w-4 text-faint')} />} />
        ))}
        <Row icon={<span>🧩</span>} title="Personality quiz" sub={`${Object.keys(me.personality).length} answered`} onClick={() => nav('/quiz')} right={<ChevronRight className="h-4 w-4 text-faint" />} />
      </Group>

      <SectionSheet section={section} onClose={() => setSection(null)} />
    </div>
  )
}

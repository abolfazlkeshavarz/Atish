import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft, Bell, Eye, EyeOff, HelpCircle, Lock, MessageCircle, Phone, ShieldCheck, Trash2, UserX } from 'lucide-react'
import { del, get, patch } from '@/api/client'
import type { Me, Privacy } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError } from '@/store/ui'
import { platform } from '@/platform/telegram'
import { auth } from '@/api/client'
import { Button, Group, Row, Sheet, Switch, Field } from '@/components/ui'

export default function Settings() {
  const nav = useNavigate()
  const { me, app, setMe, refresh } = useSession()
  const [blocked, setBlocked] = useState<{ id: string; display_name: string; atish_username: string }[] | null>(null)
  const [showBlocked, setShowBlocked] = useState(false)
  const [showDelete, setShowDelete] = useState(false)
  const [showPrivacy, setShowPrivacy] = useState(false)
  const [confirmText, setConfirmText] = useState('')
  const [busy, setBusy] = useState(false)
  const [phoneHelp, setPhoneHelp] = useState(false)

  useEffect(() => {
    if (showBlocked) get<{ items: typeof blocked }>('/api/me/blocks').then((r) => setBlocked(r.items)).catch(toastError)
  }, [showBlocked]) // eslint-disable-line react-hooks/exhaustive-deps

  if (!me) return null

  const setPrivacy = async (p: Partial<Privacy>) => {
    const next = { ...me.privacy, ...p }
    setMe({ ...me, privacy: next }) // optimistic
    try { setMe(await patch<Me>('/api/me/preferences', { privacy: next })) } catch (e) { toastError(e); void refresh() }
  }

  const verifyPhone = async () => {
    if (!platform.canRequestContact()) return setPhoneHelp(true)
    const shared = await platform.requestContact()
    if (!shared) return
    toast('Verifying…')
    for (let i = 0; i < 8; i++) {
      await new Promise((r) => setTimeout(r, 1200))
      const m = await refresh()
      if (m?.phone_verified) return toast('Phone verified ✅', 'success')
    }
    toast('Almost there — open the bot chat to confirm')
  }

  const deleteAccount = async () => {
    setBusy(true)
    try {
      await del('/api/me', { confirm: 'DELETE' })
      auth.set(null)
      location.replace('/')
    } catch (e) { toastError(e); setBusy(false) }
  }

  return (
    <div className="px-4 pb-10" style={{ paddingTop: 'calc(14px + var(--safe-t))' }}>
      <div className="mb-4 flex items-center gap-3">
        <button onClick={() => nav(-1)} className="press flex h-10 w-10 items-center justify-center rounded-full bg-surface shadow-soft" aria-label="Back"><ArrowLeft className="h-5 w-5" /></button>
        <h1 className="text-[28px] font-extrabold">Settings</h1>
      </div>

      <h2 className="mb-2 px-1 text-xs font-bold uppercase tracking-wide text-muted">Account</h2>
      <Group>
        <Row icon={<span>✈️</span>} title="Telegram" sub="Verified ✓" />
        <Row icon={<Phone className="h-5 w-5" />} title="Phone number" sub={me.phone_verified ? 'Verified ✓ — never shown to anyone' : 'Optional · adds a trust badge'}
          right={me.phone_verified
            ? <button className="press text-sm font-semibold text-danger" onClick={async () => { try { await del('/api/me/phone'); await refresh() } catch (e) { toastError(e) } }}>Remove</button>
            : <Button size="sm" onClick={verifyPhone}>Verify</Button>} />
      </Group>

      <h2 className="mb-2 px-1 text-xs font-bold uppercase tracking-wide text-muted">Visibility</h2>
      <Group>
        <Row icon={me.privacy.discoverable ? <Eye className="h-5 w-5" /> : <EyeOff className="h-5 w-5" />} title="Show me in Discover" sub="Turn off to pause your profile — matches can still chat"
          right={<Switch checked={me.privacy.discoverable} onChange={(v) => setPrivacy({ discoverable: v })} />} />
        <Row icon={<Lock className="h-5 w-5" />} title="Show my area" sub="Off = only your city is visible" right={<Switch checked={me.privacy.show_area} onChange={(v) => setPrivacy({ show_area: v })} />} />
        <Row icon={<span>🙋</span>} title="Show my gender" right={<Switch checked={me.privacy.show_gender} onChange={(v) => setPrivacy({ show_gender: v })} />} />
        <Row icon={<span>🎓</span>} title="Show work & education" right={<Switch checked={me.privacy.show_education} onChange={(v) => setPrivacy({ show_education: v })} />} />
        <Row icon={<span>🌿</span>} title="Show lifestyle" right={<Switch checked={me.privacy.show_lifestyle} onChange={(v) => setPrivacy({ show_lifestyle: v })} />} />
      </Group>

      <h2 className="mb-2 px-1 text-xs font-bold uppercase tracking-wide text-muted">Notifications (via Telegram)</h2>
      <Group>
        <Row icon={<Bell className="h-5 w-5" />} title="New matches" right={<Switch checked={me.privacy.notify_matches} onChange={(v) => setPrivacy({ notify_matches: v })} />} />
        <Row icon={<MessageCircle className="h-5 w-5" />} title="New messages" right={<Switch checked={me.privacy.notify_messages} onChange={(v) => setPrivacy({ notify_messages: v })} />} />
      </Group>

      <h2 className="mb-2 px-1 text-xs font-bold uppercase tracking-wide text-muted">Safety & help</h2>
      <Group>
        <Row icon={<UserX className="h-5 w-5" />} title="Blocked people" onClick={() => setShowBlocked(true)} />
        <Row icon={<ShieldCheck className="h-5 w-5" />} title="How we protect your privacy" onClick={() => setShowPrivacy(true)} />
        <Row icon={<HelpCircle className="h-5 w-5" />} title="Support" sub={app?.support_username ? '@' + app.support_username.replace('@', '') : 'Use /support in the Atish bot'}
          onClick={() => app?.support_username && platform.openTelegramLink(`https://t.me/${app.support_username.replace('@', '')}`)} />
      </Group>

      <Group>
        <Row icon={<Trash2 className="h-5 w-5" />} title="Delete my account" sub="Permanently removes your profile, photos, matches and messages" danger onClick={() => setShowDelete(true)} />
      </Group>
      <p className="text-center text-xs text-faint">Atish · {me.atish_username}</p>

      <Sheet open={showBlocked} onClose={() => setShowBlocked(false)} title="Blocked people">
        {blocked === null ? <p className="py-6 text-center text-muted">Loading…</p> : blocked.length === 0 ? <p className="py-8 text-center text-muted">You haven't blocked anyone.</p> : (
          <div className="space-y-2">
            {blocked.map((b) => (
              <div key={b.id} className="flex items-center justify-between rounded-2xl border border-line bg-surface p-3.5">
                <div><div className="font-bold">{b.display_name}</div><div className="text-xs text-muted">{b.atish_username}</div></div>
                <Button size="sm" variant="soft" onClick={async () => { try { await del(`/api/users/${b.id}/block`); setBlocked((x) => x?.filter((y) => y.id !== b.id) ?? null) } catch (e) { toastError(e) } }}>Unblock</Button>
              </div>
            ))}
          </div>
        )}
      </Sheet>

      <Sheet open={showPrivacy} onClose={() => setShowPrivacy(false)} title="Your privacy">
        <ul className="space-y-3 pb-2 text-[15px]">
          <li>📍 <b>Area only.</b> We never store GPS coordinates or street addresses — just the city/area you pick.</li>
          <li>📱 <b>Your phone number</b> is encrypted and never visible to anyone. It is only used for the verified badge.</li>
          <li>🆔 <b>Your Telegram ID</b> and internal IDs are never shown to other users.</li>
          <li>🎂 <b>Your birthday</b> stays private — others only see your age.</li>
          <li>🖼️ <b>Photos</b> are cleaned of hidden metadata and served through expiring links.</li>
          <li>🗑️ <b>You're in control.</b> Pause, hide, block, or delete everything anytime.</li>
        </ul>
      </Sheet>

      <Sheet open={phoneHelp} onClose={() => setPhoneHelp(false)} title="Verify your phone">
        <p className="pb-4 text-[15px] text-muted">Open the Atish bot chat in Telegram, send <b className="text-ink">/verify</b> and tap “Share my number”. Your number is encrypted and never shown to others.</p>
      </Sheet>

      <Sheet
        open={showDelete}
        onClose={() => { setShowDelete(false); setConfirmText('') }}
        title="Delete account"
        footer={<Button block variant="danger" disabled={confirmText !== 'DELETE'} loading={busy} onClick={deleteAccount}>Delete everything</Button>}
      >
        <div className="space-y-3 pb-2 text-[15px]">
          <p className="font-semibold text-danger">This cannot be undone.</p>
          <ul className="list-disc space-y-1.5 pl-5 text-muted">
            <li>Your profile, photos, interests and preferences are deleted</li>
            <li>Your likes, passes, matches and all messages are deleted</li>
            <li>Your phone number is erased and your Telegram link is removed</li>
            <li>Your Atish username becomes available again</li>
            <li>Active Plus subscriptions stop renewing</li>
          </ul>
          <Field label="Type DELETE to confirm">
            <input className="field" value={confirmText} onChange={(e) => setConfirmText(e.target.value)} autoCapitalize="characters" />
          </Field>
        </div>
      </Sheet>
    </div>
  )
}

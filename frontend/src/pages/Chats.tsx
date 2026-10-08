import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Ban, Flag, MoreVertical, Send, UserX } from 'lucide-react'
import { get, post, del } from '@/api/client'
import type { Chat, Message, PublicProfile } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError } from '@/store/ui'
import { Avatar, Button, Confirm, EmptyState, Skeleton, cx, useInterval } from '@/components/ui'
import { ProfileSheet, ReportSheet, VerifiedBadges } from '@/components/profile'
import { timeAgo } from '@/lib/format'

/* ───────── list ───────── */

export function ChatList() {
  const nav = useNavigate()
  const loadUnread = useSession((s) => s.loadUnread)
  const [chats, setChats] = useState<Chat[] | null>(null)

  const load = useCallback(async () => {
    try { setChats((await get<{ items: Chat[] }>('/api/chats')).items) } catch (e) { toastError(e); setChats((c) => c ?? []) }
  }, [])
  useEffect(() => { void load(); void loadUnread() }, [load, loadUnread])
  useInterval(() => { void load(); void loadUnread() }, 8000)

  return (
    <div className="px-4 pb-6" style={{ paddingTop: 'calc(14px + var(--safe-t))' }}>
      <h1 className="mb-4 text-[28px] font-extrabold">Chats</h1>
      {chats === null ? (
        <div className="space-y-3">{[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-[72px]" />)}</div>
      ) : chats.length === 0 ? (
        <EmptyState emoji="💬" title="No chats yet" text="Match with someone and say hello — your conversations will appear here." action={<Button onClick={() => nav('/')}>Find people</Button>} />
      ) : (
        <div className="overflow-hidden rounded-xl2 border border-line/70 bg-surface shadow-soft">
          {chats.map((c) => {
            const u = c.match.user
            const last = c.last_message
            return (
              <button key={c.match.id} onClick={() => nav(`/chats/${c.match.id}`)} className="press flex w-full items-center gap-3.5 border-b border-line/60 px-4 py-3.5 text-left last:border-0 active:bg-elevated/60">
                <Avatar src={u.photos[0]?.url} name={u.display_name} size={56} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="truncate text-base font-bold">{u.display_name}</span>
                    <span className="shrink-0 text-xs text-faint">{timeAgo(last?.created_at ?? c.match.created_at)}</span>
                  </div>
                  <div className="flex items-center justify-between gap-2">
                    <span className={cx('truncate text-sm', c.unread ? 'font-semibold text-ink' : 'text-muted')}>
                      {last ? (last.sender_id === useSession.getState().me?.id ? 'You: ' : '') + last.body : '✨ New match — say hi!'}
                    </span>
                    {c.unread > 0 && <span className="brand-gradient flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-[11px] font-bold text-white">{c.unread}</span>}
                  </div>
                </div>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}

/* ───────── thread ───────── */

export function ChatThread() {
  const { id = '' } = useParams()
  const nav = useNavigate()
  const meId = useSession((s) => s.me?.id)
  const loadUnread = useSession((s) => s.loadUnread)
  const [other, setOther] = useState<PublicProfile | null>(null)
  const [msgs, setMsgs] = useState<Message[]>([])
  const [ready, setReady] = useState(false)
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [menu, setMenu] = useState(false)
  const [profile, setProfile] = useState<PublicProfile | null>(null)
  const [report, setReport] = useState(false)
  const [confirm, setConfirm] = useState<'unmatch' | 'block' | null>(null)
  const bottom = useRef<HTMLDivElement>(null)
  const lastId = useRef(0)

  const append = useCallback((incoming: Message[]) => {
    if (!incoming.length) return
    setMsgs((cur) => {
      const have = new Set(cur.map((m) => m.id))
      const merged = [...cur.filter((m) => m.id > 0), ...incoming.filter((m) => !have.has(m.id))]
      lastId.current = merged.reduce((a, m) => Math.max(a, m.id), 0)
      return merged
    })
  }, [])

  useEffect(() => {
    let live = true
    ;(async () => {
      try {
        const [chats, m] = await Promise.all([get<{ items: Chat[] }>('/api/chats'), get<{ items: Message[] }>(`/api/chats/${id}/messages`)])
        if (!live) return
        const c = chats.items.find((x) => x.match.id === id)
        if (!c) return nav('/chats', { replace: true })
        setOther(c.match.user)
        append(m.items)
        setReady(true)
        post(`/api/chats/${id}/read`).then(() => loadUnread()).catch(() => {})
      } catch (e) { toastError(e); nav('/chats', { replace: true }) }
    })()
    return () => { live = false }
  }, [id, nav, append, loadUnread])

  // lightweight polling keeps the chat live without needing WebSockets
  useInterval(async () => {
    if (!ready || document.hidden) return
    try {
      const r = await get<{ items: Message[] }>(`/api/chats/${id}/messages`, { after: lastId.current })
      if (r.items.length) {
        append(r.items)
        if (r.items.some((m) => m.sender_id !== meId)) post(`/api/chats/${id}/read`).catch(() => {})
      }
    } catch (e) {
      if ((e as { status?: number }).status === 404) nav('/chats', { replace: true })
    }
  }, ready ? 3000 : null)

  useEffect(() => { bottom.current?.scrollIntoView({ behavior: 'smooth', block: 'end' }) }, [msgs.length])

  const send = async () => {
    const body = text.trim()
    if (!body || sending) return
    setSending(true)
    setText('')
    try {
      const m = await post<Message>(`/api/chats/${id}/messages`, { body })
      append([m])
    } catch (e) { setText(body); toastError(e) } finally { setSending(false) }
  }

  const openProfile = async () => {
    if (!other) return
    try { setProfile(await get<PublicProfile>(`/api/users/${other.id}`)) } catch { setProfile(other) }
  }

  if (!other) return <div className="p-4 pt-16"><Skeleton className="h-16" /></div>

  return (
    <div className="mx-auto flex h-full max-w-xl flex-col">
      <header className="glass z-10 flex items-center gap-2 border-b border-line/60 px-2 pb-2.5" style={{ paddingTop: 'calc(10px + var(--safe-t))' }}>
        <button onClick={() => nav('/chats')} className="press flex h-10 w-10 items-center justify-center rounded-full" aria-label="Back"><ArrowLeft className="h-6 w-6" /></button>
        <button onClick={openProfile} className="press flex min-w-0 flex-1 items-center gap-3 text-left">
          <Avatar src={other.photos[0]?.url} name={other.display_name} size={42} />
          <div className="min-w-0">
            <div className="truncate font-extrabold leading-tight">{other.display_name} <VerifiedBadges p={other} size={15} /></div>
            <div className="truncate text-xs text-muted">{other.city}{other.area ? ` · ${other.area}` : ''}</div>
          </div>
        </button>
        <div className="relative">
          <button onClick={() => setMenu((m) => !m)} className="press flex h-10 w-10 items-center justify-center rounded-full" aria-label="More"><MoreVertical className="h-5 w-5" /></button>
          {menu && (
            <div className="absolute right-0 top-11 z-20 w-48 animate-pop overflow-hidden rounded-2xl border border-line bg-surface shadow-card" onClick={() => setMenu(false)}>
              <button className="press flex w-full items-center gap-2 px-4 py-3 text-sm font-semibold" onClick={() => setConfirm('unmatch')}><UserX className="h-4 w-4" /> Unmatch</button>
              <button className="press flex w-full items-center gap-2 px-4 py-3 text-sm font-semibold" onClick={() => setReport(true)}><Flag className="h-4 w-4" /> Report</button>
              <button className="press flex w-full items-center gap-2 px-4 py-3 text-sm font-semibold text-danger" onClick={() => setConfirm('block')}><Ban className="h-4 w-4" /> Block</button>
            </div>
          )}
        </div>
      </header>

      <div className="scroll-hide flex-1 overflow-y-auto px-3 py-4" onClick={() => setMenu(false)}>
        {ready && msgs.length === 0 && (
          <div className="mx-auto mt-10 max-w-xs animate-rise text-center">
            <Avatar src={other.photos[0]?.url} name={other.display_name} size={88} className="mx-auto mb-3 ring-4 ring-brand/30" />
            <h3 className="text-lg font-extrabold">You matched with {other.display_name}!</h3>
            <p className="mt-1 text-sm text-muted">Break the ice — ask about something you both like.</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2">
              {['Hey! 👋', 'Hi, how’s your week going?', 'What do you do for fun?'].map((s) => (
                <button key={s} onClick={() => setText(s)} className="press rounded-full border border-line bg-surface px-3 py-1.5 text-sm font-medium">{s}</button>
              ))}
            </div>
          </div>
        )}
        {msgs.map((m, i) => {
          const mine = m.sender_id === meId
          const nextSame = msgs[i + 1]?.sender_id === m.sender_id
          return (
            <div key={m.id} className={cx('flex', mine ? 'justify-end' : 'justify-start', nextSame ? 'mb-1' : 'mb-3')}>
              <div
                className={cx(
                  'max-w-[78%] whitespace-pre-wrap break-words px-4 py-2.5 text-[15px] leading-snug shadow-soft',
                  mine ? 'brand-gradient rounded-[20px] rounded-br-md text-white' : 'rounded-[20px] rounded-bl-md border border-line/60 bg-surface',
                )}
              >
                {m.body}
                {!nextSame && <div className={cx('mt-1 text-[10px]', mine ? 'text-white/70' : 'text-faint')}>{new Date(m.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}{mine && m.read_at ? ' · seen' : ''}</div>}
              </div>
            </div>
          )
        })}
        <div ref={bottom} />
      </div>

      <form
        onSubmit={(e) => { e.preventDefault(); void send() }}
        className="glass flex items-center gap-2 border-t border-line/60 px-3 pt-2.5"
        style={{ paddingBottom: 'calc(10px + var(--safe-b))' }}
      >
        <input
          className="field !rounded-full !py-3"
          placeholder="Message…"
          value={text}
          maxLength={2000}
          onChange={(e) => setText(e.target.value)}
          enterKeyHint="send"
        />
        <button type="submit" disabled={!text.trim() || sending} className="press brand-gradient flex h-12 w-12 shrink-0 items-center justify-center rounded-full text-white shadow-glow disabled:opacity-40" aria-label="Send">
          <Send className="h-5 w-5" />
        </button>
      </form>

      <ProfileSheet p={profile} onClose={() => setProfile(null)} onChanged={(k) => { setProfile(null); if (k === 'blocked') nav('/chats', { replace: true }) }} />
      <ReportSheet open={report} onClose={() => setReport(false)} userId={other.id} name={other.display_name} />
      <Confirm
        open={confirm === 'unmatch'} title={`Unmatch ${other.display_name}?`} text="The conversation will be closed for both of you." confirmLabel="Unmatch" danger onClose={() => setConfirm(null)}
        onConfirm={async () => { try { await del(`/api/matches/${id}`); toast('Unmatched'); nav('/chats', { replace: true }) } catch (e) { toastError(e) } }}
      />
      <Confirm
        open={confirm === 'block'} title={`Block ${other.display_name}?`} text="They won't be able to contact you or see your profile." confirmLabel="Block" danger onClose={() => setConfirm(null)}
        onConfirm={async () => { try { await post(`/api/users/${other.id}/block`); toast('Blocked', 'success'); nav('/chats', { replace: true }) } catch (e) { toastError(e) } }}
      />
    </div>
  )
}

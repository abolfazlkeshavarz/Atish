import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Lock } from 'lucide-react'
import { get, post } from '@/api/client'
import type { Match, PublicProfile, SwipeResult } from '@/api/types'
import { useSession } from '@/store/session'
import { toastError, useUI } from '@/store/ui'
import { Button, EmptyState, Skeleton } from '@/components/ui'
import { IntentBadges, MatchOverlay, ProfileSheet } from '@/components/profile'

interface Likes { total: number; locked: boolean; profiles: PublicProfile[] }

export default function Matches() {
  const nav = useNavigate()
  const me = useSession((s) => s.me)
  const openPremium = useUI((s) => s.openPremium)
  const [matches, setMatches] = useState<Match[] | null>(null)
  const [likes, setLikes] = useState<Likes | null>(null)
  const [open, setOpen] = useState<PublicProfile | null>(null)
  const [celebrate, setCelebrate] = useState<{ user: PublicProfile; id: string } | null>(null)

  const load = useCallback(async () => {
    try {
      const [m, l] = await Promise.all([get<{ items: Match[] }>('/api/matches'), get<Likes>('/api/likes/received')])
      setMatches(m.items)
      setLikes(l)
    } catch (e) { toastError(e); setMatches((x) => x ?? []) }
  }, [])
  useEffect(() => { void load() }, [load])

  const respond = async (p: PublicProfile, like: boolean) => {
    setOpen(null)
    setLikes((l) => l && { ...l, total: Math.max(0, l.total - 1), profiles: l.profiles.filter((x) => x.id !== p.id) })
    try {
      if (!like) return void (await post(`/api/discover/${p.id}/pass`))
      const r = await post<SwipeResult>(`/api/discover/${p.id}/like`)
      if (r.matched && r.user && r.match_id) { setCelebrate({ user: r.user, id: r.match_id }); void load() }
    } catch (e) {
      const err = e as { code?: string }
      if (err.code === 'daily_limit') openPremium('limit')
      else toastError(e)
    }
  }

  const fresh = (matches ?? []).filter((m) => !m.last_message_at)
  const older = (matches ?? []).filter((m) => m.last_message_at)

  return (
    <div className="px-4 pb-6" style={{ paddingTop: 'calc(14px + var(--safe-t))' }}>
      <h1 className="mb-4 text-[28px] font-extrabold">Matches</h1>

      {likes && likes.total > 0 && (
        <div className="mb-6">
          <div className="mb-2 flex items-center justify-between px-1">
            <h2 className="text-[15px] font-bold">Likes you <span className="ml-1 rounded-full bg-brand/15 px-2 py-0.5 text-xs text-brand">{likes.total}</span></h2>
            {likes.locked && <button onClick={() => openPremium('see_likes')} className="text-xs font-bold text-brand">See all</button>}
          </div>
          {likes.locked ? (
            <button onClick={() => openPremium('see_likes')} className="press relative w-full overflow-hidden rounded-xl2 brand-gradient p-5 text-left text-white shadow-glow">
              <div className="flex items-center gap-4">
                <div className="flex -space-x-4">
                  {Array.from({ length: Math.min(3, likes.total) }).map((_, i) => (
                    <div key={i} className="h-14 w-14 rounded-full border-2 border-white/80 bg-white/30 backdrop-blur-md" />
                  ))}
                </div>
                <div className="flex-1">
                  <div className="text-lg font-extrabold">{likes.total} {likes.total === 1 ? 'person likes' : 'people like'} you</div>
                  <div className="flex items-center gap-1 text-sm text-white/85"><Lock className="h-3.5 w-3.5" /> Unlock with Atish Plus</div>
                </div>
              </div>
            </button>
          ) : (
            <div className="scroll-hide -mx-4 flex gap-3 overflow-x-auto px-4">
              {likes.profiles.map((p) => (
                <button key={p.id} onClick={() => setOpen(p)} className="press relative h-44 w-32 shrink-0 overflow-hidden rounded-2xl shadow-soft">
                  {p.photos[0] && <img src={p.photos[0].url} alt="" className="h-full w-full object-cover" />}
                  <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent p-2.5 text-left text-white">
                    <div className="truncate text-sm font-bold">{p.display_name}, {p.age}</div>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {matches === null ? (
        <div className="grid grid-cols-2 gap-3">{[0, 1, 2, 3].map((i) => <Skeleton key={i} className="aspect-[3/4]" />)}</div>
      ) : matches.length === 0 ? (
        <EmptyState emoji="💫" title="No matches yet" text="When you and someone else are both interested, they show up here. Keep swiping!" action={<Button onClick={() => nav('/')}>Discover people</Button>} />
      ) : (
        <>
          {fresh.length > 0 && (
            <>
              <h2 className="mb-2 px-1 text-[15px] font-bold">New matches</h2>
              <div className="mb-6 grid grid-cols-2 gap-3">
                {fresh.map((m) => <MatchTile key={m.id} m={m} isNew onClick={() => nav(`/chats/${m.id}`)} />)}
              </div>
            </>
          )}
          {older.length > 0 && (
            <>
              <h2 className="mb-2 px-1 text-[15px] font-bold">Conversations</h2>
              <div className="grid grid-cols-2 gap-3">
                {older.map((m) => <MatchTile key={m.id} m={m} onClick={() => nav(`/chats/${m.id}`)} />)}
              </div>
            </>
          )}
        </>
      )}

      <ProfileSheet
        p={open}
        onClose={() => setOpen(null)}
        onChanged={() => { setOpen(null); void load() }}
        actions={open && (
          <div className="flex gap-3">
            <Button variant="outline" size="lg" block onClick={() => respond(open, false)}>✕ Pass</Button>
            <Button size="lg" block onClick={() => respond(open, true)}>❤️ Like back</Button>
          </div>
        )}
      />
      {celebrate && <MatchOverlay other={celebrate.user} me={me?.id} onClose={() => setCelebrate(null)} onChat={() => { const id = celebrate.id; setCelebrate(null); nav(`/chats/${id}`) }} />}
    </div>
  )
}

function MatchTile({ m, isNew, onClick }: { m: Match; isNew?: boolean; onClick: () => void }) {
  const p = m.user
  return (
    <button onClick={onClick} className="press relative aspect-[3/4] overflow-hidden rounded-2xl bg-elevated text-left shadow-soft">
      {p.photos[0] ? <img src={p.photos[0].url} alt="" className="h-full w-full object-cover" loading="lazy" /> : <div className="ph-bg h-full w-full" />}
      <div className="absolute inset-0 bg-gradient-to-t from-black/75 via-transparent to-transparent" />
      {isNew && <span className="brand-gradient absolute left-2.5 top-2.5 rounded-full px-2 py-0.5 text-[10px] font-extrabold text-white">NEW</span>}
      <div className="absolute inset-x-0 bottom-0 p-3 text-white">
        <div className="truncate text-base font-extrabold">{p.display_name}, {p.age}</div>
        <IntentBadges types={m.connection_types} className="mt-1 [&>span]:!px-2 [&>span]:!py-0.5 [&>span]:!text-[10px]" />
      </div>
    </button>
  )
}


/* eslint-disable @typescript-eslint/no-explicit-any */
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Plus, Trash2 } from 'lucide-react'
import { Badge, Button, Card, ChipGroup, Confirm, Field, Modal, Sheet, Spinner, Switch, cx, useDebounced } from '@/components/ui'
import { toast, toastError } from '@/store/ui'
import { money } from '@/lib/format'
import { useAdmin } from './AdminApp'

/* ───────── shared bits ───────── */

function useFetch<T = any>(path: string, query?: Record<string, string | number | undefined>, deps: unknown[] = []) {
  const { call } = useAdmin()
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const key = JSON.stringify(query)
  const reload = useCallback(async () => {
    setLoading(true)
    try { setData((await call('GET', path, undefined, query)) as T) } catch (e) { toastError(e) } finally { setLoading(false) }
  }, [call, path, key]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { void reload() }, [reload, ...deps]) // eslint-disable-line react-hooks/exhaustive-deps
  return { data, loading, reload, setData }
}

function Title({ children, sub, right }: { children: ReactNode; sub?: string; right?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
      <div><h1 className="text-2xl font-extrabold md:text-3xl">{children}</h1>{sub && <p className="mt-1 text-sm text-muted">{sub}</p>}</div>
      {right}
    </div>
  )
}

interface Col<R> { h: string; cell: (r: R) => ReactNode; cls?: string }
function Table<R>({ cols, rows, onRow, empty = 'Nothing here yet' }: { cols: Col<R>[]; rows: R[]; onRow?: (r: R) => void; empty?: string }) {
  return (
    <div className="overflow-x-auto rounded-xl2 border border-line/70 bg-surface shadow-soft">
      <table className="w-full min-w-[640px] text-left text-sm">
        <thead><tr className="border-b border-line/70 bg-elevated/50 text-xs uppercase tracking-wide text-muted">{cols.map((c) => <th key={c.h} className={cx('px-4 py-3 font-bold', c.cls)}>{c.h}</th>)}</tr></thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} onClick={() => onRow?.(r)} className={cx('border-b border-line/50 last:border-0', onRow && 'cursor-pointer hover:bg-elevated/50')}>
              {cols.map((c) => <td key={c.h} className={cx('px-4 py-3 align-middle', c.cls)}>{c.cell(r)}</td>)}
            </tr>
          ))}
          {rows.length === 0 && <tr><td colSpan={cols.length} className="px-4 py-10 text-center text-muted">{empty}</td></tr>}
        </tbody>
      </table>
    </div>
  )
}

function Pager({ page, setPage, total, per = 25 }: { page: number; setPage: (p: number) => void; total?: number; per?: number }) {
  const pages = total ? Math.max(1, Math.ceil(total / per)) : undefined
  return (
    <div className="mt-4 flex items-center justify-between text-sm text-muted">
      <span>{total !== undefined ? `${total} total` : ''}</span>
      <div className="flex items-center gap-2">
        <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => setPage(page - 1)}>Prev</Button>
        <span>Page {page}{pages ? ` / ${pages}` : ''}</span>
        <Button size="sm" variant="outline" disabled={pages ? page >= pages : false} onClick={() => setPage(page + 1)}>Next</Button>
      </div>
    </div>
  )
}

const statusTone = (s: string): 'ok' | 'brand' | 'muted' | 'friend' =>
  ({ active: 'ok', suspended: 'brand', banned: 'brand', deleted: 'muted', open: 'brand', resolved: 'ok', dismissed: 'muted', reviewing: 'friend', paid: 'ok', pending: 'muted', failed: 'brand', canceled: 'muted', expired: 'muted' } as any)[s] ?? 'muted'

const when = (iso?: string | null) => (iso ? new Date(iso).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) : '—')

/* ───────── Dashboard ───────── */

export function Dashboard() {
  const { data: s, loading } = useFetch<any>('/admin/stats')
  if (loading || !s) return <div className="flex justify-center py-20"><Spinner className="h-8 w-8" /></div>
  const cards: [string, ReactNode, string?][] = [
    ['Users', s.users_total], ['Onboarded', s.users_onboarded], ['Active (24h)', s.active_24h], ['Active (7d)', s.active_7d],
    ['New (24h)', s.new_24h], ['Restricted', s.restricted], ['Matches', s.matches_total], ['Matches (24h)', s.matches_24h],
    ['Messages', s.messages_total], ['Messages (24h)', s.messages_24h], ['Swipes (24h)', s.swipes_24h],
    ['Open reports', s.reports_open, s.reports_open > 0 ? 'text-brand' : ''], ['Plus members', s.premium_users],
    ['Revenue (30d)', money(s.revenue_30d_cents, 'usd')], ['Stars (30d)', `⭐ ${s.stars_30d}`],
  ]
  const days: any[] = s.daily ?? []
  const max = Math.max(1, ...days.map((d) => Math.max(d.signups, d.matches)))
  return (
    <div>
      <Title sub="Live overview of the platform">Dashboard</Title>
      <div className="mb-8 grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5">
        {cards.map(([k, v, cls]) => (
          <Card key={k} className="p-4"><div className="text-xs font-semibold text-muted">{k}</div><div className={cx('mt-1 text-2xl font-extrabold', cls)}>{v}</div></Card>
        ))}
      </div>
      <Card className="p-5">
        <div className="mb-4 flex items-center justify-between"><h3 className="font-bold">Last 14 days</h3>
          <div className="flex gap-4 text-xs text-muted"><span><i className="mr-1 inline-block h-2 w-2 rounded-full bg-brand" />Sign-ups</span><span><i className="mr-1 inline-block h-2 w-2 rounded-full bg-friend" />Matches</span></div></div>
        <div className="flex h-40 items-end gap-2">
          {days.map((d) => (
            <div key={d.day} className="flex flex-1 flex-col items-center gap-1" title={`${d.day}: ${d.signups} sign-ups, ${d.matches} matches`}>
              <div className="flex h-32 w-full items-end justify-center gap-0.5">
                <div className="w-1/2 rounded-t bg-brand" style={{ height: `${(d.signups / max) * 100}%`, minHeight: d.signups ? 3 : 0 }} />
                <div className="w-1/2 rounded-t bg-friend" style={{ height: `${(d.matches / max) * 100}%`, minHeight: d.matches ? 3 : 0 }} />
              </div>
              <span className="text-[10px] text-faint">{d.day.slice(8)}</span>
            </div>
          ))}
        </div>
      </Card>
    </div>
  )
}

/* ───────── Users ───────── */

export function UsersPage() {
  const nav = useNavigate()
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const [role, setRole] = useState('')
  const [premium, setPremium] = useState('')
  const [page, setPage] = useState(1)
  const dq = useDebounced(q, 350)
  useEffect(() => setPage(1), [dq, status, role, premium])
  const { data, loading } = useFetch<{ items: any[]; total: number }>('/admin/users', { q: dq, status, role, premium, page })
  const sel = 'field !w-auto !py-2.5 text-sm'
  return (
    <div>
      <Title sub="Search by Atish username, name, Telegram username/ID or user ID">Users</Title>
      <div className="mb-4 flex flex-wrap gap-2">
        <input className="field !w-full !py-2.5 md:!w-80" placeholder="Search users…" value={q} onChange={(e) => setQ(e.target.value)} />
        <select className={sel} value={status} onChange={(e) => setStatus(e.target.value)}><option value="">Any status</option><option>active</option><option>suspended</option><option>banned</option><option>deleted</option></select>
        <select className={sel} value={role} onChange={(e) => setRole(e.target.value)}><option value="">Any role</option><option>user</option><option>moderator</option><option>admin</option></select>
        <select className={sel} value={premium} onChange={(e) => setPremium(e.target.value)}><option value="">Plus: any</option><option value="yes">Plus members</option><option value="no">Free</option></select>
      </div>
      {loading && !data ? <div className="flex justify-center py-16"><Spinner /></div> : (
        <>
          <Table
            rows={data?.items ?? []}
            onRow={(u) => nav(`/admin/users/${u.id}`)}
            cols={[
              { h: 'User', cell: (u) => <div><div className="font-bold">{u.display_name || <i className="text-faint">no profile</i>}</div><div className="text-xs text-muted">{u.atish_username ?? '—'}</div></div> },
              { h: 'Where', cell: (u) => (u.city ? `${u.city}, ${u.country_code}` : '—') },
              { h: 'Status', cell: (u) => <div className="flex flex-wrap gap-1"><Badge tone={statusTone(u.status)}>{u.status}</Badge>{u.role !== 'user' && <Badge tone="friend">{u.role}</Badge>}{u.premium && <Badge>Plus</Badge>}{u.open_reports > 0 && <Badge tone="brand">⚑ {u.open_reports}</Badge>}</div> },
              { h: 'Verified', cell: (u) => <span className="text-xs">{u.telegram_verified && '✈️ '}{u.phone_verified && '📱 '}{u.photo_verified && '📷 '}{u.identity_verified && '🪪'}</span> },
              { h: 'Last active', cell: (u) => <span className="text-xs text-muted">{when(u.last_active_at)}</span> },
            ]}
          />
          <Pager page={page} setPage={setPage} total={data?.total} />
        </>
      )}
    </div>
  )
}

export function UserDetailPage() {
  const { id } = useParams()
  const nav = useNavigate()
  const { role, call } = useAdmin()
  const { data: d, loading, reload } = useFetch<any>(`/admin/users/${id}`)
  const [reason, setReason] = useState('')
  const [bio, setBio] = useState<string | null>(null)
  const [name, setName] = useState<string | null>(null)
  const [del, setDel] = useState(false)
  const [grant, setGrant] = useState(false)
  const [plans, setPlans] = useState<any[]>([])
  const [plan, setPlan] = useState('premium_month')
  const [days, setDays] = useState(30)

  useEffect(() => { if (grant) call('GET', '/admin/catalog/plans', undefined, { per: 50 }).then((r: any) => setPlans(r.items)).catch(toastError) }, [grant, call])
  if (loading && !d) return <div className="flex justify-center py-20"><Spinner className="h-8 w-8" /></div>
  if (!d) return null
  const p = d.profile
  const run = async (fn: () => Promise<unknown>, ok = 'Done') => { try { await fn(); toast(ok, 'success'); await reload() } catch (e) { toastError(e) } }
  const setUser = (fields: Record<string, unknown>) => run(() => call('PATCH', `/admin/users/${id}`, fields), 'Updated')
  const ex = d.extras

  return (
    <div>
      <button onClick={() => nav(-1)} className="press mb-4 flex items-center gap-1.5 text-sm font-semibold text-muted"><ArrowLeft className="h-4 w-4" /> Back</button>
      <Title sub={`${p.atish_username ?? 'no username'} · created ${when(p.created_at)}`}
        right={<div className="flex gap-2"><Badge tone={statusTone(p.status)}>{p.status}</Badge><Badge tone="friend">{p.role}</Badge>{d.entitlements.premium && <Badge>Plus</Badge>}</div>}>
        {p.display_name || 'Unfinished profile'}
      </Title>

      <div className="grid gap-6 lg:grid-cols-[1.3fr_1fr]">
        <div className="space-y-6">
          <Card className="p-5">
            <h3 className="mb-3 font-bold">Photos</h3>
            <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
              {p.photos.map((ph: any) => (
                <div key={ph.id} className="group relative aspect-[3/4] overflow-hidden rounded-xl bg-elevated">
                  <img src={ph.url} alt="" className="h-full w-full object-cover" />
                  <button onClick={() => run(() => call('DELETE', `/admin/photos/${ph.id}`), 'Photo removed')} className="absolute right-1.5 top-1.5 rounded-full bg-danger p-1.5 text-white opacity-0 transition group-hover:opacity-100" aria-label="Remove photo"><Trash2 className="h-4 w-4" /></button>
                </div>
              ))}
              {p.photos.length === 0 && <p className="col-span-full text-sm text-muted">No photos</p>}
            </div>
          </Card>

          <Card className="p-5">
            <h3 className="mb-3 font-bold">Profile</h3>
            <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
              {([
                ['Age', p.age ? `${p.age} (born ${p.birth_date})` : '—'], ['Gender', p.gender || '—'],
                ['Location', p.location ? `${p.location.city}${p.location.area ? ' · ' + p.location.area : ''}, ${p.location.country}` : '—'],
                ['Looking for', p.connection_types.join(', ') || '—'], ['Distance', p.preferences.distance_scope], ['Age range', `${p.preferences.age_min}–${p.preferences.age_max}`],
                ['Telegram ID', d.telegram_user_id ?? '—'], ['Telegram username', d.telegram_username ? '@' + d.telegram_username : '—'],
              ] as [string, any][]).map(([k, v]) => <div key={k}><dt className="text-xs font-semibold text-muted">{k}</dt><dd className="font-medium">{v}</dd></div>)}
            </dl>
            <div className="mt-4"><dt className="text-xs font-semibold text-muted">Bio</dt>
              {bio === null ? <p className="mt-1 whitespace-pre-line text-sm">{p.bio || '—'} <button className="ml-2 text-xs font-bold text-brand" onClick={() => { setBio(p.bio); setName(p.display_name) }}>Edit text</button></p> : (
                <div className="mt-2 space-y-2">
                  <input className="field" value={name ?? ''} onChange={(e) => setName(e.target.value)} />
                  <textarea className="field min-h-[80px]" value={bio} onChange={(e) => setBio(e.target.value)} />
                  <div className="flex gap-2"><Button size="sm" onClick={() => run(async () => { await call('PATCH', `/admin/users/${id}/profile`, { display_name: name, bio }); setBio(null) }, 'Saved')}>Save</Button><Button size="sm" variant="soft" onClick={() => setBio(null)}>Cancel</Button></div>
                </div>
              )}
            </div>
            <div className="mt-4 flex flex-wrap gap-1.5">{p.interests.map((i: string) => <Badge key={i} tone="muted">{i}</Badge>)}</div>
            <div className="mt-3 flex flex-wrap gap-1.5">{p.languages.map((l: any) => <Badge key={l.code} tone="friend">{l.code} · {l.level}</Badge>)}</div>
          </Card>

          <Card className="p-5">
            <h3 className="mb-3 font-bold">Activity</h3>
            <div className="grid grid-cols-3 gap-3 text-center sm:grid-cols-4">
              {([['Matches', ex.matches], ['Likes sent', ex.likes_sent], ['Likes received', ex.likes_received], ['Messages', ex.messages_sent], ['Blocked by', ex.blocked_by], ['Reports against', ex.reports_against], ['Reports filed', ex.reports_filed]] as [string, number][]).map(([k, v]) => (
                <div key={k} className="rounded-xl bg-elevated/60 p-3"><div className="text-xl font-extrabold">{v}</div><div className="text-[11px] text-muted">{k}</div></div>
              ))}
            </div>
            <p className="mt-3 text-xs text-muted">Last active {when(p.last_active_at)}</p>
          </Card>
        </div>

        <div className="space-y-6">
          <Card className="p-5">
            <h3 className="mb-3 font-bold">Moderation</h3>
            <Field label="Reason (shown to the user)"><input className="field" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="e.g. Fake photos" /></Field>
            <div className="grid grid-cols-3 gap-2">
              <Button size="sm" variant="outline" disabled={p.status === 'active'} onClick={() => setUser({ status: 'active', status_reason: '' })}>Activate</Button>
              <Button size="sm" variant="outline" disabled={p.status === 'suspended'} onClick={() => setUser({ status: 'suspended', status_reason: reason })}>Suspend</Button>
              <Button size="sm" variant="danger" disabled={p.status === 'banned'} onClick={() => setUser({ status: 'banned', status_reason: reason })}>Ban</Button>
            </div>
            {p.status_reason && <p className="mt-2 text-xs text-muted">Current reason: {p.status_reason}</p>}
          </Card>

          <Card className="p-5">
            <h3 className="mb-3 font-bold">Verification badges</h3>
            {(['telegram_verified', 'phone_verified', 'photo_verified', 'identity_verified'] as const).map((k) => (
              <div key={k} className="flex items-center justify-between py-2"><span className="text-sm font-medium capitalize">{k.replace('_verified', '')}</span><Switch checked={p[k]} onChange={(v) => setUser({ [k]: v })} /></div>
            ))}
          </Card>

          {role === 'admin' && (
            <>
              <Card className="p-5">
                <h3 className="mb-3 font-bold">Role</h3>
                <ChipGroup options={['user', 'moderator', 'admin'].map((r) => ({ v: r, label: r }))} value={p.role} onChange={(r) => setUser({ role: r })} />
              </Card>
              <Card className="p-5">
                <h3 className="mb-1 font-bold">Atish Plus</h3>
                <p className="mb-3 text-sm text-muted">{d.entitlements.premium ? `Active (${d.subscription?.provider}) until ${d.entitlements.until ? when(d.entitlements.until) : 'forever'}` : 'Not subscribed'}</p>
                <div className="flex gap-2"><Button size="sm" onClick={() => setGrant(true)}>Grant Plus</Button>
                  {d.entitlements.premium && <Button size="sm" variant="outline" onClick={() => run(() => call('DELETE', `/admin/users/${id}/premium`), 'Revoked')}>Revoke</Button>}</div>
              </Card>
              <Card className="border-danger/40 p-5">
                <h3 className="mb-1 font-bold text-danger">Danger zone</h3>
                <p className="mb-3 text-sm text-muted">Deletes profile, photos, matches and messages. This is irreversible.</p>
                <Button size="sm" variant="danger" onClick={() => setDel(true)}>Delete account</Button>
              </Card>
            </>
          )}
        </div>
      </div>

      <Modal open={grant} onClose={() => setGrant(false)}>
        <h3 className="mb-4 text-lg font-extrabold">Grant Atish Plus</h3>
        <Field label="Plan"><select className="field" value={plan} onChange={(e) => setPlan(e.target.value)}>{plans.map((x) => <option key={x.code} value={x.code}>{x.name}</option>)}</select></Field>
        <Field label="Days (0 = lifetime)"><input className="field" type="number" min={0} value={days} onChange={(e) => setDays(+e.target.value)} /></Field>
        <Button block onClick={() => run(async () => { await call('POST', `/admin/users/${id}/premium`, { plan, days }); setGrant(false) }, 'Granted')}>Grant</Button>
      </Modal>
      <Confirm open={del} danger title="Delete this account?" text="This permanently erases the user's data." confirmLabel="Delete" onClose={() => setDel(false)}
        onConfirm={async () => { try { await call('DELETE', `/admin/users/${id}`); toast('Deleted', 'success'); nav('/admin/users') } catch (e) { toastError(e) } }} />
    </div>
  )
}

/* ───────── Reports ───────── */

export function ReportsPage() {
  const [status, setStatus] = useState('open')
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<number | null>(null)
  const { data, loading, reload } = useFetch<{ items: any[] }>('/admin/reports', { status, page })
  return (
    <div>
      <Title sub="Review user reports and take action">Reports</Title>
      <div className="mb-4"><ChipGroup options={['open', 'reviewing', 'resolved', 'dismissed', ''].map((s) => ({ v: s, label: s || 'all' }))} value={status} onChange={(v) => { setStatus(v); setPage(1) }} /></div>
      {loading && !data ? <div className="flex justify-center py-16"><Spinner /></div> : (
        <>
          <Table rows={data?.items ?? []} onRow={(r) => setOpen(r.id)} empty="No reports 🎉" cols={[
            { h: '#', cell: (r) => r.id },
            { h: 'Reported', cell: (r) => <div><div className="font-bold">{r.reported_username}</div><div className="text-xs text-muted">{r.reported_total_reports} total report(s) · {r.reported_status}</div></div> },
            { h: 'Reason', cell: (r) => <div><div className="font-medium">{String(r.reason).replace(/_/g, ' ')}</div><div className="max-w-xs truncate text-xs text-muted">{r.details}</div></div> },
            { h: 'By', cell: (r) => r.reporter_username },
            { h: 'Status', cell: (r) => <Badge tone={statusTone(r.status)}>{r.status}</Badge> },
            { h: 'Filed', cell: (r) => <span className="text-xs text-muted">{when(r.created_at)}</span> },
          ]} />
          <Pager page={page} setPage={setPage} />
        </>
      )}
      {open !== null && <ReportDrawer id={open} onClose={() => setOpen(null)} onChanged={reload} />}
    </div>
  )
}

function ReportDrawer({ id, onClose, onChanged }: { id: number; onClose: () => void; onChanged: () => void }) {
  const { call } = useAdmin()
  const { data, loading } = useFetch<any>(`/admin/reports/${id}`)
  const [note, setNote] = useState('')
  const r = data?.report
  const act = async (status: string, action = 'none') => {
    try { await call('PATCH', `/admin/reports/${id}`, { status, resolution: note, action }); toast('Report updated', 'success'); onChanged(); onClose() } catch (e) { toastError(e) }
  }
  return (
    <Sheet open onClose={onClose} title={`Report #${id}`} tall footer={r && (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Button size="sm" variant="outline" onClick={() => act('dismissed')}>Dismiss</Button>
        <Button size="sm" variant="soft" onClick={() => act('reviewing')}>Mark reviewing</Button>
        <Button size="sm" variant="outline" onClick={() => act('resolved', 'suspend')}>Suspend user</Button>
        <Button size="sm" variant="danger" onClick={() => act('resolved', 'ban')}>Ban user</Button>
      </div>
    )}>
      {loading || !r ? <Spinner /> : (
        <div className="space-y-4 pb-2 text-sm">
          <div className="rounded-xl bg-elevated/60 p-4">
            <div className="mb-1 flex items-center gap-2"><Badge tone={statusTone(r.status)}>{r.status}</Badge><b>{String(r.reason).replace(/_/g, ' ')}</b></div>
            <p className="text-muted">{r.details || 'No details provided'}</p>
            <p className="mt-2 text-xs text-muted">Reported: <Link className="font-bold text-brand" to={`/admin/users/${r.reported_id}`}>{r.reported_username}</Link> ({r.reported_status}) · by {r.reporter_username} · {when(r.created_at)}</p>
          </div>
          <Field label="Resolution note (also shown to the user if suspended/banned)"><textarea className="field min-h-[70px]" value={note} onChange={(e) => setNote(e.target.value)} /></Field>
          <div>
            <h4 className="mb-2 font-bold">Conversation <span className="text-xs font-normal text-muted">(access is logged)</span></h4>
            {data.conversation.length === 0 ? <p className="text-muted">No conversation attached.</p> : (
              <div className="space-y-1.5 rounded-xl border border-line/70 bg-surface p-3">
                {data.conversation.map((m: any) => (
                  <div key={m.id} className={cx('rounded-lg px-3 py-2', m.sender_id === r.reported_id ? 'bg-brand/10' : 'bg-elevated/70')}>
                    <div className="text-[11px] font-bold text-muted">{m.sender_username} · {when(m.created_at)}</div>
                    <div className="whitespace-pre-wrap">{m.body}</div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </Sheet>
  )
}

/* ───────── Photos ───────── */

export function PhotosPage() {
  const { call } = useAdmin()
  const [page, setPage] = useState(1)
  const { data, loading, reload } = useFetch<{ items: any[] }>('/admin/photos', { page, per: 48 })
  return (
    <div>
      <Title sub="Newest uploads first — remove anything that breaks the rules">Photo moderation</Title>
      {loading && !data ? <div className="flex justify-center py-16"><Spinner /></div> : (
        <>
          <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 lg:grid-cols-6">
            {(data?.items ?? []).map((p) => (
              <div key={p.id} className="group relative aspect-[3/4] overflow-hidden rounded-xl bg-elevated shadow-soft">
                <img src={p.url} alt="" loading="lazy" className="h-full w-full object-cover" />
                <Link to={`/admin/users/${p.user_id}`} className="absolute inset-x-0 bottom-0 truncate bg-gradient-to-t from-black/80 to-transparent p-2 text-[11px] font-semibold text-white">{p.atish_username}</Link>
                <button onClick={async () => { try { await call('DELETE', `/admin/photos/${p.id}`); toast('Removed', 'success'); void reload() } catch (e) { toastError(e) } }}
                  className="absolute right-1.5 top-1.5 rounded-full bg-danger p-1.5 text-white opacity-0 transition group-hover:opacity-100" aria-label="Remove"><Trash2 className="h-4 w-4" /></button>
              </div>
            ))}
          </div>
          <Pager page={page} setPage={setPage} />
        </>
      )}
    </div>
  )
}

/* ───────── Catalog CRUD (interests, languages, plans, locations…) ───────── */

type FT = 'text' | 'int' | 'bool' | 'json' | 'select'
interface FDef { k: string; label: string; t?: FT; opts?: string[]; nullable?: boolean; list?: boolean; pk?: boolean }
const RES: Record<string, { title: string; hint: string; pk: string; auto: boolean; fields: FDef[] }> = {
  interests: { title: 'Interests', hint: 'Predefined interest options users pick from', pk: 'id', auto: true, fields: [
    { k: 'emoji', label: 'Emoji', list: true }, { k: 'name', label: 'Name', list: true }, { k: 'slug', label: 'Slug (unique)', list: true }, { k: 'category', label: 'Category', list: true },
    { k: 'position', label: 'Order', t: 'int' }, { k: 'is_active', label: 'Active', t: 'bool', list: true }] },
  languages: { title: 'Languages', hint: 'Languages available in profiles (code = ISO 639-1)', pk: 'code', auto: false, fields: [
    { k: 'code', label: 'Code', pk: true, list: true }, { k: 'name', label: 'Name', list: true }, { k: 'native_name', label: 'Native name', list: true }, { k: 'is_active', label: 'Active', t: 'bool', list: true }] },
  'connection-types': { title: 'Connection types', hint: 'What people can look for. Built-in rules use "friends" and "relationship"; new types match only on identical selection.', pk: 'slug', auto: false, fields: [
    { k: 'slug', label: 'Slug', pk: true, list: true }, { k: 'emoji', label: 'Emoji', list: true }, { k: 'name', label: 'Name', list: true }, { k: 'position', label: 'Order', t: 'int', list: true }, { k: 'is_active', label: 'Active', t: 'bool', list: true }] },
  'friendship-kinds': { title: 'Friendship kinds', hint: 'Sub-options shown when someone wants friends', pk: 'slug', auto: false, fields: [
    { k: 'slug', label: 'Slug', pk: true, list: true }, { k: 'emoji', label: 'Emoji', list: true }, { k: 'name', label: 'Name', list: true }, { k: 'position', label: 'Order', t: 'int', list: true }, { k: 'is_active', label: 'Active', t: 'bool', list: true }] },
  'personality-questions': { title: 'Quiz questions', hint: 'Lightweight compatibility questions (options as JSON: [{"key":"a","label":"…"}])', pk: 'id', auto: true, fields: [
    { k: 'key', label: 'Key (unique)', list: true }, { k: 'text', label: 'Question', list: true }, { k: 'options', label: 'Options JSON', t: 'json' }, { k: 'position', label: 'Order', t: 'int', list: true }, { k: 'is_active', label: 'Active', t: 'bool', list: true }] },
  locations: { title: 'Locations', hint: 'Country › region › city › area. No coordinates are ever stored.', pk: 'id', auto: true, fields: [
    { k: 'country_code', label: 'Country (ISO-2)', list: true }, { k: 'country', label: 'Country name', list: true }, { k: 'region', label: 'Region', list: true }, { k: 'city', label: 'City', list: true }, { k: 'area', label: 'Area (empty = city)', list: true }, { k: 'is_approved', label: 'Approved', t: 'bool', list: true }] },
  plans: { title: 'Plans', hint: 'Premium plans. Stars price enables Telegram Stars; set a Stripe price id for card subscriptions. Prices in minor units (cents).', pk: 'id', auto: true, fields: [
    { k: 'code', label: 'Code (unique)', list: true }, { k: 'name', label: 'Name', list: true }, { k: 'description', label: 'Description' }, { k: 'interval', label: 'Interval', t: 'select', opts: ['month', 'year', 'lifetime'], list: true },
    { k: 'price_cents', label: 'Price (cents)', t: 'int', list: true }, { k: 'currency', label: 'Currency', list: true }, { k: 'stars_price', label: 'Stars price', t: 'int', nullable: true, list: true },
    { k: 'stripe_price_id', label: 'Stripe price id', nullable: true }, { k: 'features', label: 'Features JSON', t: 'json' }, { k: 'position', label: 'Order', t: 'int' }, { k: 'is_active', label: 'Active', t: 'bool', list: true }] },
}

function fieldValue(f: FDef, row: any): string {
  const v = row?.[f.k]
  if (f.t === 'json') return v === undefined ? (f.k === 'options' ? '[]' : '{}') : JSON.stringify(v, null, 2)
  if (f.t === 'bool') return String(v ?? true)
  return v === null || v === undefined ? '' : String(v)
}

export function CatalogPage() {
  const { resource = '' } = useParams()
  const cfg = RES[resource]
  const { call } = useAdmin()
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const dq = useDebounced(q, 300)
  const [edit, setEdit] = useState<any | 'new' | null>(null)
  const [rm, setRm] = useState<any | null>(null)
  useEffect(() => { setPage(1); setQ('') }, [resource])
  const { data, loading, reload } = useFetch<{ items: any[]; total: number }>(`/admin/catalog/${resource}`, { q: dq, page, per: 50 }, [resource])
  if (!cfg) return <p>Unknown resource</p>
  const listCols = cfg.fields.filter((f) => f.list)
  return (
    <div>
      <Title sub={cfg.hint} right={<Button size="sm" onClick={() => setEdit('new')}><Plus className="h-4 w-4" /> Add</Button>}>{cfg.title}</Title>
      <input className="field mb-4 !w-full !py-2.5 md:!w-72" placeholder="Search…" value={q} onChange={(e) => setQ(e.target.value)} />
      {loading && !data ? <div className="flex justify-center py-16"><Spinner /></div> : (
        <>
          <Table rows={data?.items ?? []} onRow={setEdit} cols={[
            ...listCols.map((f) => ({ h: f.label, cell: (r: any) => f.t === 'bool' ? (r[f.k] ? <Badge tone="ok">yes</Badge> : <Badge tone="muted">no</Badge>) : <span className="max-w-[240px] truncate">{String(r[f.k] ?? '')}</span> })),
            { h: '', cls: 'w-12', cell: (r: any) => <button className="text-faint hover:text-danger" onClick={(e) => { e.stopPropagation(); setRm(r) }} aria-label="Delete"><Trash2 className="h-4 w-4" /></button> },
          ]} />
          <Pager page={page} setPage={setPage} total={data?.total} per={50} />
        </>
      )}
      {edit !== null && <CatalogForm cfg={cfg} row={edit === 'new' ? null : edit} onClose={() => setEdit(null)}
        onSave={async (body) => {
          try {
            if (edit === 'new') await call('POST', `/admin/catalog/${resource}`, body)
            else await call('PATCH', `/admin/catalog/${resource}/${edit[cfg.pk]}`, body)
            toast('Saved', 'success'); setEdit(null); void reload()
          } catch (e) { toastError(e) }
        }} />}
      <Confirm open={!!rm} danger title="Delete this item?" text="If it is already in use, deactivate it instead." confirmLabel="Delete" onClose={() => setRm(null)}
        onConfirm={async () => { try { await call('DELETE', `/admin/catalog/${resource}/${rm[cfg.pk]}`); toast('Deleted', 'success'); setRm(null); void reload() } catch (e) { toastError(e); setRm(null) } }} />
    </div>
  )
}

function CatalogForm({ cfg, row, onClose, onSave }: { cfg: (typeof RES)[string]; row: any | null; onClose: () => void; onSave: (b: Record<string, unknown>) => Promise<void> }) {
  const fields = cfg.fields.filter((f) => !f.pk || !row) // primary keys are only editable on create
  const [vals, setVals] = useState<Record<string, string>>(() => Object.fromEntries(cfg.fields.map((f) => [f.k, row ? fieldValue(f, row) : f.t === 'bool' ? 'true' : f.t === 'json' ? fieldValue(f, null) : f.t === 'int' && !f.nullable ? '0' : ''])))
  const [busy, setBusy] = useState(false)
  const submit = async () => {
    const body: Record<string, unknown> = {}
    try {
      for (const f of fields) {
        const v = vals[f.k]
        if (f.t === 'int') body[f.k] = v === '' ? (f.nullable ? null : 0) : Number(v)
        else if (f.t === 'bool') body[f.k] = v === 'true'
        else if (f.t === 'json') body[f.k] = JSON.parse(v || 'null')
        else body[f.k] = f.nullable && v === '' ? null : v
      }
    } catch { toast('Invalid JSON', 'error'); return }
    setBusy(true); await onSave(body); setBusy(false)
  }
  return (
    <Sheet open onClose={onClose} title={row ? 'Edit' : 'Add'} tall footer={<Button block loading={busy} onClick={submit}>Save</Button>}>
      {fields.map((f) => (
        <Field key={f.k} label={f.label}>
          {f.t === 'bool' ? <div className="py-1"><Switch checked={vals[f.k] === 'true'} onChange={(v) => setVals({ ...vals, [f.k]: String(v) })} /></div>
            : f.t === 'json' ? <textarea className="field min-h-[140px] font-mono text-xs" value={vals[f.k]} onChange={(e) => setVals({ ...vals, [f.k]: e.target.value })} />
            : f.t === 'select' ? <select className="field" value={vals[f.k]} onChange={(e) => setVals({ ...vals, [f.k]: e.target.value })}>{f.opts!.map((o) => <option key={o}>{o}</option>)}</select>
            : <input className="field" type={f.t === 'int' ? 'number' : 'text'} value={vals[f.k]} onChange={(e) => setVals({ ...vals, [f.k]: e.target.value })} />}
        </Field>
      ))}
    </Sheet>
  )
}

/* ───────── Billing ───────── */

export function BillingPage() {
  const [tab, setTab] = useState<'payments' | 'subscriptions'>('payments')
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const { data, loading } = useFetch<{ items: any[] }>(`/admin/${tab}`, { status, page }, [tab])
  return (
    <div>
      <Title sub="Provider-agnostic: Telegram Stars, Stripe, manual grants">Payments & subscriptions</Title>
      <div className="mb-3"><ChipGroup options={[{ v: 'payments', label: 'Payments' }, { v: 'subscriptions', label: 'Subscriptions' }]} value={tab} onChange={(v) => { setTab(v as any); setStatus(''); setPage(1) }} /></div>
      <div className="mb-4"><ChipGroup options={(tab === 'payments' ? ['', 'paid', 'pending', 'failed', 'refunded'] : ['', 'active', 'canceled', 'expired']).map((s) => ({ v: s, label: s || 'all' }))} value={status} onChange={(v) => { setStatus(v); setPage(1) }} /></div>
      {loading && !data ? <div className="flex justify-center py-16"><Spinner /></div> : (
        <>
          {tab === 'payments' ? (
            <Table rows={data?.items ?? []} cols={[
              { h: 'User', cell: (r) => <Link className="font-bold text-brand" to={`/admin/users/${r.user_id}`}>{r.atish_username ?? r.user_id.slice(0, 8)}</Link> },
              { h: 'Plan', cell: (r) => r.plan }, { h: 'Provider', cell: (r) => r.provider },
              { h: 'Amount', cell: (r) => (r.currency === 'XTR' ? `⭐ ${r.amount_cents}` : money(r.amount_cents, r.currency)) },
              { h: 'Status', cell: (r) => <Badge tone={statusTone(r.status)}>{r.status}</Badge> }, { h: 'Date', cell: (r) => <span className="text-xs text-muted">{when(r.paid_at ?? r.created_at)}</span> },
            ]} />
          ) : (
            <Table rows={data?.items ?? []} cols={[
              { h: 'User', cell: (r) => <Link className="font-bold text-brand" to={`/admin/users/${r.user_id}`}>{r.atish_username ?? r.user_id.slice(0, 8)}</Link> },
              { h: 'Plan', cell: (r) => r.plan }, { h: 'Provider', cell: (r) => r.provider }, { h: 'Status', cell: (r) => <Badge tone={statusTone(r.status)}>{r.status}</Badge> },
              { h: 'Started', cell: (r) => <span className="text-xs text-muted">{when(r.started_at)}</span> }, { h: 'Renews / ends', cell: (r) => <span className="text-xs text-muted">{r.current_period_end ? when(r.current_period_end) : 'lifetime'}</span> },
            ]} />
          )}
          <Pager page={page} setPage={setPage} />
        </>
      )}
    </div>
  )
}

/* ───────── Settings ───────── */

const SETTINGS: { k: string; label: string; hint: string; t: 'int' | 'bool' | 'text' | 'words' | 'json' }[] = [
  { k: 'premium_enabled', label: 'Premium enabled', hint: 'Turn off to make every feature free for everyone (free-launch mode).', t: 'bool' },
  { k: 'free_daily_likes', label: 'Free daily likes', hint: 'Likes per day for non-Plus users.', t: 'int' },
  { k: 'registration_open', label: 'Registration open', hint: 'Disable to stop new sign-ups.', t: 'bool' },
  { k: 'maintenance_mode', label: 'Maintenance mode', hint: 'Locks the app for non-staff users.', t: 'bool' },
  { k: 'min_age', label: 'Minimum age', hint: 'Users younger than this cannot register.', t: 'int' },
  { k: 'max_photos', label: 'Max photos per profile', hint: '1–9', t: 'int' },
  { k: 'discovery_batch_size', label: 'Discovery batch size', hint: 'Profiles loaded per request.', t: 'int' },
  { k: 'pass_resurface_days', label: 'Passed profiles return after (days)', hint: 'Passed profiles may reappear after this many days.', t: 'int' },
  { k: 'banner', label: 'Announcement banner', hint: 'Shown at the top of the app. Empty = hidden.', t: 'text' },
  { k: 'support_username', label: 'Support Telegram username', hint: 'Without @. Shown in Settings and /support.', t: 'text' },
  { k: 'blocked_words', label: 'Blocked words', hint: 'One per line. Applied to usernames, names, bios and place names.', t: 'words' },
  { k: 'score_weights', label: 'Matching weights', hint: 'JSON. Connection intent should stay the largest weight.', t: 'json' },
]

export function SettingsPage() {
  const { call } = useAdmin()
  const { data, loading, reload } = useFetch<Record<string, any>>('/admin/settings')
  const [draft, setDraft] = useState<Record<string, any>>({})
  useEffect(() => { if (data) setDraft(data) }, [data])
  if (loading && !data) return <div className="flex justify-center py-16"><Spinner /></div>
  const save = async (k: string, t: string) => {
    let v = draft[k]
    try {
      if (t === 'words' && typeof v === 'string') v = v.split('\n').map((s) => s.trim()).filter(Boolean)
      if (t === 'json' && typeof v === 'string') v = JSON.parse(v)
      if (t === 'int') v = Number(v)
      await call('PUT', `/admin/settings/${k}`, { value: v })
      toast('Saved', 'success'); void reload()
    } catch (e) { toastError(e instanceof SyntaxError ? new Error('Invalid JSON') : e) }
  }
  return (
    <div>
      <Title sub="Changes apply within seconds, no deploy needed">Platform settings</Title>
      <div className="space-y-3">
        {SETTINGS.map((s) => {
          const v = draft[s.k]
          return (
            <Card key={s.k} className="flex flex-col gap-3 p-4 md:flex-row md:items-center">
              <div className="md:w-1/3"><div className="font-bold">{s.label}</div><div className="text-xs text-muted">{s.hint}</div></div>
              <div className="flex flex-1 items-center gap-3">
                {s.t === 'bool' ? <Switch checked={!!v} onChange={async (b) => { setDraft({ ...draft, [s.k]: b }); try { await call('PUT', `/admin/settings/${s.k}`, { value: b }); toast('Saved', 'success') } catch (e) { toastError(e); void reload() } }} />
                  : s.t === 'words' ? <textarea className="field min-h-[110px] font-mono text-xs" value={Array.isArray(v) ? v.join('\n') : v ?? ''} onChange={(e) => setDraft({ ...draft, [s.k]: e.target.value })} />
                  : s.t === 'json' ? <textarea className="field min-h-[110px] font-mono text-xs" value={typeof v === 'string' ? v : JSON.stringify(v ?? {}, null, 2)} onChange={(e) => setDraft({ ...draft, [s.k]: e.target.value })} />
                  : <input className="field" type={s.t === 'int' ? 'number' : 'text'} value={v ?? ''} onChange={(e) => setDraft({ ...draft, [s.k]: e.target.value })} />}
                {s.t !== 'bool' && <Button size="sm" onClick={() => save(s.k, s.t)}>Save</Button>}
              </div>
            </Card>
          )
        })}
      </div>
    </div>
  )
}

/* ───────── Broadcast ───────── */

export function BroadcastPage() {
  const { call } = useAdmin()
  const [text, setText] = useState('')
  const [confirm, setConfirm] = useState(false)
  return (
    <div className="max-w-2xl">
      <Title sub="Send a message to every active user through the Telegram bot">Broadcast</Title>
      <Card className="p-5">
        <textarea className="field min-h-[160px]" maxLength={1000} placeholder="Write your announcement…" value={text} onChange={(e) => setText(e.target.value)} />
        <div className="mt-2 text-right text-xs text-muted">{text.length}/1000</div>
        <Button className="mt-3" disabled={!text.trim()} onClick={() => setConfirm(true)}>Send to everyone</Button>
      </Card>
      <Confirm open={confirm} title="Send to all users?" text="This cannot be undone. Messages are sent gradually to respect Telegram's limits." confirmLabel="Send" onClose={() => setConfirm(false)}
        onConfirm={async () => { try { const r: any = await call('POST', '/admin/broadcast', { text }); toast(`Sending to ${r.recipients} users`, 'success'); setText(''); setConfirm(false) } catch (e) { toastError(e); setConfirm(false) } }} />
    </div>
  )
}

/* ───────── Audit ───────── */

export function AuditPage() {
  const [page, setPage] = useState(1)
  const { data, loading } = useFetch<{ items: any[] }>('/admin/audit', { page, per: 50 })
  return (
    <div>
      <Title sub="Every administrative action is recorded">Audit log</Title>
      {loading && !data ? <div className="flex justify-center py-16"><Spinner /></div> : (
        <>
          <Table rows={data?.items ?? []} cols={[
            { h: 'When', cell: (r) => <span className="whitespace-nowrap text-xs text-muted">{when(r.created_at)}</span> },
            { h: 'Actor', cell: (r) => <span className="font-mono text-xs">{String(r.actor).slice(0, 8)}</span> },
            { h: 'Action', cell: (r) => <Badge tone="friend">{r.action}</Badge> },
            { h: 'Target', cell: (r) => <span className="font-mono text-xs">{r.target_type} {String(r.target_id ?? '').slice(0, 12)}</span> },
            { h: 'Details', cell: (r) => <span className="block max-w-xs truncate font-mono text-xs text-muted">{r.details ? JSON.stringify(r.details) : ''}</span> },
          ]} />
          <Pager page={page} setPage={setPage} />
        </>
      )}
    </div>
  )
}

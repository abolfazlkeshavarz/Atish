import { Logo } from '@/components/Logo'
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { NavLink, Navigate, Route, Routes, useNavigate } from 'react-router-dom'
import {
  BarChart3, BookOpen, CreditCard, Flag, Image as ImageIcon, LogOut, Megaphone, Menu, ScrollText, Settings as Cog, Tags, Users as UsersIcon, X,
} from 'lucide-react'
import { ApiError, api } from '@/api/client'
import { platform } from '@/platform/telegram'
import { Button, Field, cx } from '@/components/ui'
import { toastError } from '@/store/ui'
import { Dashboard, UsersPage, UserDetailPage, ReportsPage, PhotosPage, CatalogPage, BillingPage, SettingsPage, BroadcastPage, AuditPage } from './pages'

const KEY = 'atish.admin.token'

export interface AdminCtx {
  role: string
  actor: string
  call(method: string, path: string, body?: unknown, query?: Record<string, string | number | undefined>): Promise<any>
}
const Ctx = createContext<AdminCtx>(null as unknown as AdminCtx)
export const useAdmin = () => useContext(Ctx)

export default function AdminApp() {
  const [token, setToken] = useState<string | null>(() => {
    try { return localStorage.getItem(KEY) } catch { return null }
  })
  const [who, setWho] = useState<{ role: string; actor: string } | null>(null)
  const [checking, setChecking] = useState(true)

  const logout = useCallback(() => {
    try { localStorage.removeItem(KEY) } catch { /* ignore */ }
    setToken(null); setWho(null)
  }, [])

  const call = useCallback(async (method: string, path: string, body?: unknown, query?: Record<string, string | number | undefined>) => {
    try {
      return await api(method, '/api' + path, { token, body, query, noAuth: false })
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) logout()
      throw e
    }
  }, [token, logout])

  // Validate any stored token; inside Telegram, try signing in as an admin automatically.
  useEffect(() => {
    let live = true
    ;(async () => {
      try {
        let t = token
        if (!t && platform.inTelegram()) {
          const r = await api<{ token: string }>('POST', '/api/auth/telegram', { body: { init_data: platform.initData() }, noAuth: true })
          t = r.token
        }
        if (!t) return
        const w = await api<{ role: string; actor: string }>('GET', '/api/admin/whoami', { token: t })
        if (!live) return
        try { localStorage.setItem(KEY, t) } catch { /* ignore */ }
        setToken(t); setWho(w)
      } catch {
        if (live && token) logout()
      } finally {
        if (live) setChecking(false)
      }
    })()
    return () => { live = false }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const value = useMemo<AdminCtx | null>(() => (who ? { role: who.role, actor: who.actor, call } : null), [who, call])

  if (checking) return <div className="flex h-full items-center justify-center text-muted">Loading…</div>
  if (!value) return <Login onDone={(t, w) => { try { localStorage.setItem(KEY, t) } catch { /* ignore */ } setToken(t); setWho(w) }} />

  return (
    <Ctx.Provider value={value}>
      <Shell onLogout={logout}>
        <Routes>
          <Route path="/admin" element={<Dashboard />} />
          <Route path="/admin/users" element={<UsersPage />} />
          <Route path="/admin/users/:id" element={<UserDetailPage />} />
          <Route path="/admin/reports" element={<ReportsPage />} />
          <Route path="/admin/photos" element={<PhotosPage />} />
          <Route path="/admin/catalog/:resource" element={<CatalogPage />} />
          <Route path="/admin/billing" element={<BillingPage />} />
          <Route path="/admin/settings" element={<SettingsPage />} />
          <Route path="/admin/broadcast" element={<BroadcastPage />} />
          <Route path="/admin/audit" element={<AuditPage />} />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Routes>
      </Shell>
    </Ctx.Provider>
  )
}

function Login({ onDone }: { onDone: (token: string, who: { role: string; actor: string }) => void }) {
  const [u, setU] = useState('admin')
  const [p, setP] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true); setErr('')
    try {
      const r = await api<{ token: string }>('POST', '/api/admin/login', { body: { username: u, password: p }, noAuth: true })
      const w = await api<{ role: string; actor: string }>('GET', '/api/admin/whoami', { token: r.token })
      onDone(r.token, w)
    } catch (e2) { setErr((e2 as Error).message) } finally { setBusy(false) }
  }
  return (
    <div className="flex h-full items-center justify-center p-6">
      <form onSubmit={submit} className="w-full max-w-sm rounded-xl3 border border-line/70 bg-surface p-7 shadow-card">
        <div className="mb-6 flex items-center gap-3">
          <Logo tile size={48} glow />
          <div><div className="text-xl font-black">Atish Admin</div><div className="text-xs text-muted">Sign in to manage the platform</div></div>
        </div>
        <Field label="Username" hint="admin, or your own Atish username if you were given a password"><input className="field" value={u} onChange={(e) => setU(e.target.value)} autoComplete="username" /></Field>
        <Field label="Password" error={err}><input className="field" type="password" value={p} onChange={(e) => setP(e.target.value)} autoComplete="current-password" autoFocus /></Field>
        <Button block size="lg" loading={busy} disabled={!p}>Sign in</Button>
      </form>
    </div>
  )
}

const NAV: { to: string; label: string; icon: ReactNode; admin?: boolean; end?: boolean }[] = [
  { to: '/admin', label: 'Dashboard', icon: <BarChart3 className="h-[18px] w-[18px]" />, end: true },
  { to: '/admin/users', label: 'Users', icon: <UsersIcon className="h-[18px] w-[18px]" /> },
  { to: '/admin/reports', label: 'Reports', icon: <Flag className="h-[18px] w-[18px]" /> },
  { to: '/admin/photos', label: 'Photos', icon: <ImageIcon className="h-[18px] w-[18px]" /> },
  { to: '/admin/catalog/interests', label: 'Interests', icon: <Tags className="h-[18px] w-[18px]" />, admin: true },
  { to: '/admin/catalog/languages', label: 'Languages', icon: <BookOpen className="h-[18px] w-[18px]" />, admin: true },
  { to: '/admin/catalog/connection-types', label: 'Connection types', icon: <span className="text-base leading-none">🤝</span>, admin: true },
  { to: '/admin/catalog/friendship-kinds', label: 'Friendship kinds', icon: <span className="text-base leading-none">🎮</span>, admin: true },
  { to: '/admin/catalog/personality-questions', label: 'Quiz questions', icon: <span className="text-base leading-none">🧩</span>, admin: true },
  { to: '/admin/catalog/locations', label: 'Locations', icon: <span className="text-base leading-none">📍</span>, admin: true },
  { to: '/admin/catalog/plans', label: 'Plans', icon: <CreditCard className="h-[18px] w-[18px]" />, admin: true },
  { to: '/admin/billing', label: 'Payments', icon: <span className="text-base leading-none">💳</span>, admin: true },
  { to: '/admin/settings', label: 'Settings', icon: <Cog className="h-[18px] w-[18px]" />, admin: true },
  { to: '/admin/broadcast', label: 'Broadcast', icon: <Megaphone className="h-[18px] w-[18px]" />, admin: true },
  { to: '/admin/audit', label: 'Audit log', icon: <ScrollText className="h-[18px] w-[18px]" /> },
]

function Shell({ children, onLogout }: { children: ReactNode; onLogout: () => void }) {
  const { role } = useAdmin()
  const [open, setOpen] = useState(false)
  const nav = useNavigate()
  useEffect(() => { document.title = 'Atish Admin' }, [])
  const items = NAV.filter((n) => !n.admin || role === 'admin')
  const menu = (
    <nav className="flex flex-col gap-0.5 p-3">
      {items.map((n) => (
        <NavLink key={n.to} to={n.to} end={n.end} onClick={() => setOpen(false)}
          className={({ isActive }) => cx('flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-semibold transition', isActive ? 'brand-gradient text-white shadow-soft' : 'text-muted hover:bg-elevated hover:text-ink')}>
          {n.icon}{n.label}
        </NavLink>
      ))}
    </nav>
  )
  return (
    <div className="flex h-full bg-bg">
      <aside className="hidden w-64 shrink-0 flex-col border-r border-line/70 bg-surface md:flex">
        <button onClick={() => nav('/admin')} className="flex items-center gap-2.5 px-5 py-5">
          <Logo tile size={36} />
          <span className="text-gradient text-xl font-black">Atish</span><span className="rounded bg-elevated px-1.5 py-0.5 text-[10px] font-bold text-muted">ADMIN</span>
        </button>
        <div className="scroll-hide flex-1 overflow-y-auto">{menu}</div>
        <button onClick={onLogout} className="m-3 flex items-center gap-2 rounded-xl px-3 py-2.5 text-sm font-semibold text-muted hover:bg-elevated"><LogOut className="h-4 w-4" /> Sign out · {role}</button>
      </aside>

      {open && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/50" onClick={() => setOpen(false)} />
          <aside className="absolute inset-y-0 left-0 flex w-72 animate-fade flex-col bg-surface shadow-card">
            <div className="flex items-center justify-between px-5 py-4"><span className="text-gradient text-xl font-black">Atish Admin</span><button onClick={() => setOpen(false)}><X className="h-5 w-5" /></button></div>
            <div className="scroll-hide flex-1 overflow-y-auto">{menu}</div>
            <button onClick={onLogout} className="m-3 flex items-center gap-2 rounded-xl px-3 py-2.5 text-sm font-semibold text-muted"><LogOut className="h-4 w-4" /> Sign out</button>
          </aside>
        </div>
      )}

      <main className="min-w-0 flex-1 overflow-y-auto">
        <div className="sticky top-0 z-30 flex items-center gap-3 border-b border-line/70 bg-bg/90 px-4 py-3 backdrop-blur md:hidden">
          <button onClick={() => setOpen(true)} className="press flex h-10 w-10 items-center justify-center rounded-xl bg-surface shadow-soft" aria-label="Menu"><Menu className="h-5 w-5" /></button>
          <span className="text-gradient text-lg font-black">Atish Admin</span>
        </div>
        <div className="mx-auto max-w-6xl p-4 md:p-8">{children}</div>
      </main>
    </div>
  )
}

export function useAdminCall() {
  const { call } = useAdmin()
  return useCallback(async <T,>(...args: Parameters<typeof call>): Promise<T | undefined> => {
    try { return (await call(...args)) as T } catch (e) { toastError(e); return undefined }
  }, [call])
}

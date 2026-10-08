import { Logo } from '@/components/Logo'
import { Suspense, lazy, useEffect, useState } from 'react'
import { Navigate, NavLink, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { Flame, Heart, MessageCircle, User } from 'lucide-react'
import { useSession } from '@/store/session'
import { platform } from '@/platform/telegram'
import { Button, Field, FullSpinner, Toasts, cx, useInterval } from '@/components/ui'
import PremiumSheet from '@/components/PremiumSheet'
import Discover from '@/pages/Discover'
import Matches from '@/pages/Matches'
import { ChatList, ChatThread } from '@/pages/Chats'
import Profile from '@/pages/Profile'
import Settings from '@/pages/Settings'
import Quiz from '@/pages/Quiz'
import Onboarding from '@/pages/Onboarding'

const AdminApp = lazy(() => import('@/admin/AdminApp'))

const DEV_LOGIN = import.meta.env.DEV || import.meta.env.VITE_DEV_AUTH === 'true'
const BOT = (import.meta.env.VITE_BOT_USERNAME as string | undefined)?.replace('@', '')

export default function App() {
  const loc = useLocation()
  if (loc.pathname.startsWith('/admin')) {
    return (
      <Suspense fallback={<FullSpinner />}>
        <AdminApp />
        <Toasts />
      </Suspense>
    )
  }
  return <UserApp />
}

function UserApp() {
  const { phase, boot, me, error, blockedReason } = useSession()
  const nav = useNavigate()
  const loc = useLocation()

  useEffect(() => { void boot() }, [boot])

  // deep links: ?go=/chats/<id> (from bot notifications) or Telegram start_param
  useEffect(() => {
    if (phase !== 'ready') return
    const go = new URLSearchParams(location.search).get('go')
    if (go && go.startsWith('/') && !go.startsWith('//')) nav(go, { replace: true })
  }, [phase, nav])

  if (phase === 'booting') return <Splash />
  if (phase === 'needs-login') return <LoginScreen />
  if (phase === 'blocked') return <Notice emoji="🚫" title="Account restricted" text={blockedReason} sub={error} />
  if (phase === 'maintenance') return <Notice emoji="🛠️" title="Be right back" text={error || 'Atish is being upgraded. Please check back in a few minutes.'} />
  if (phase === 'error') return <Notice emoji="📡" title="Can't reach Atish" text={error} action={<Button onClick={() => location.reload()}>Retry</Button>} />
  if (!me) return <Splash />

  if (!me.onboarding.complete) {
    return (<><Onboarding /><Toasts /></>)
  }

  const bare = /^\/(chats\/.+|settings|quiz)/.test(loc.pathname)
  return (
    <div className="mx-auto flex h-full max-w-xl flex-col bg-bg">
      {me.app.banner && <div className="brand-gradient px-4 py-2 text-center text-xs font-semibold text-white" style={{ paddingTop: 'calc(8px + var(--safe-t))' }}>{me.app.banner}</div>}
      <div className="min-h-0 flex-1 overflow-y-auto scroll-hide">
        <BackButton />
        <Routes>
          <Route path="/" element={<Discover />} />
          <Route path="/matches" element={<Matches />} />
          <Route path="/chats" element={<ChatList />} />
          <Route path="/chats/:id" element={<ChatThread />} />
          <Route path="/profile" element={<Profile />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/quiz" element={<Quiz />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </div>
      {!bare && <TabBar />}
      <PremiumSheet />
      <Toasts />
    </div>
  )
}

function BackButton() {
  const nav = useNavigate()
  const loc = useLocation()
  useEffect(() => {
    const root = ['/', '/matches', '/chats', '/profile'].includes(loc.pathname)
    if (root) return
    return platform.showBack(() => nav(-1))
  }, [loc.pathname, nav])
  return null
}

function TabBar() {
  const unread = useSession((s) => s.unread)
  const loadUnread = useSession((s) => s.loadUnread)
  useEffect(() => { void loadUnread() }, [loadUnread])
  useInterval(() => { if (!document.hidden) void loadUnread() }, 15000)

  const tabs = [
    { to: '/', icon: Flame, label: 'Discover', end: true },
    { to: '/matches', icon: Heart, label: 'Matches' },
    { to: '/chats', icon: MessageCircle, label: 'Chats', badge: unread },
    { to: '/profile', icon: User, label: 'Profile' },
  ]
  return (
    <nav className="glass z-30 w-full shrink-0 border-t border-line/60" style={{ paddingBottom: 'var(--safe-b)' }}>
      <div className="flex items-stretch justify-around px-2 pt-1.5">
        {tabs.map((t) => (
          <NavLink key={t.to} to={t.to} end={t.end} className="press relative flex flex-1 flex-col items-center gap-0.5 pb-2 pt-1.5" onClick={() => platform.haptic('select')}>
            {({ isActive }) => (
              <>
                <span className={cx('relative flex h-8 w-14 items-center justify-center rounded-full transition-all', isActive && 'brand-gradient shadow-glow')}>
                  <t.icon className={cx('h-[22px] w-[22px] transition-colors', isActive ? 'text-white' : 'text-muted')} fill={isActive && t.to !== '/profile' ? 'currentColor' : 'none'} />
                  {!!t.badge && t.badge > 0 && (
                    <span className="absolute -right-0.5 -top-1 flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-danger px-1 text-[10px] font-bold text-white ring-2 ring-surface">{t.badge > 9 ? '9+' : t.badge}</span>
                  )}
                </span>
                <span className={cx('text-[11px] font-semibold', isActive ? 'text-ink' : 'text-muted')}>{t.label}</span>
              </>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  )
}

/* ───────── boot screens ───────── */

function Splash() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-5">
      <div className="animate-pop"><Logo tile size={104} glow /></div>
      <div className="text-gradient text-5xl font-black tracking-tight">Atish</div>
      <div className="-mt-3 text-[11px] font-semibold uppercase tracking-[0.4em] text-muted">Real connections</div>
      <div className="h-1 w-24 overflow-hidden rounded-full bg-line"><div className="brand-gradient h-full w-1/2 animate-pulse rounded-full" /></div>
    </div>
  )
}

function Notice({ emoji, title, text, sub, action }: { emoji: string; title: string; text?: string; sub?: string; action?: React.ReactNode }) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-8 text-center">
      <div className="mb-5 text-6xl">{emoji}</div>
      <h1 className="text-2xl font-extrabold">{title}</h1>
      {text && <p className="mt-2 max-w-xs text-muted">{text}</p>}
      {sub && <p className="mt-1 max-w-xs text-sm text-faint">{sub}</p>}
      {action && <div className="mt-6">{action}</div>}
    </div>
  )
}

function LoginScreen() {
  const { devLogin } = useSession()
  const [id, setId] = useState(() => String(100000 + Math.floor(Math.random() * 900000)))
  const [name, setName] = useState('Alex')
  const [busy, setBusy] = useState(false)

  return (
    <div className="mx-auto flex h-full max-w-sm flex-col items-center justify-center px-6 text-center">
      <div className="mb-5"><Logo tile size={104} glow /></div>
      <h1 className="text-gradient text-4xl font-black">Atish</h1>
      <p className="mt-2 text-muted">Find people who are looking for the same kind of connection.</p>

      {BOT && (
        <Button size="lg" block className="mt-8" onClick={() => platform.openTelegramLink(`https://t.me/${BOT}`)}>Open in Telegram</Button>
      )}
      {!BOT && !DEV_LOGIN && <p className="mt-8 text-sm text-faint">Open Atish from the Telegram bot to continue.</p>}

      {DEV_LOGIN && (
        <div className="mt-8 w-full rounded-xl2 border border-dashed border-line p-4 text-left">
          <p className="mb-3 text-xs font-bold uppercase tracking-wide text-muted">Developer sign-in (not shown in production)</p>
          <Field label="Test Telegram ID"><input className="field" value={id} onChange={(e) => setId(e.target.value.replace(/\D/g, ''))} inputMode="numeric" /></Field>
          <Field label="Name"><input className="field" value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Button block loading={busy} onClick={async () => { setBusy(true); try { await devLogin(+id, name) } catch (e) { alert((e as Error).message) } finally { setBusy(false) } }}>
            Continue as test user
          </Button>
        </div>
      )}
    </div>
  )
}

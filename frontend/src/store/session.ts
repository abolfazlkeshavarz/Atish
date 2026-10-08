import { create } from 'zustand'
import { api, auth, setUnauthorizedHandler, ApiError, get } from '@/api/client'
import type { AppSettings, Catalog, Me } from '@/api/types'
import { platform } from '@/platform/telegram'

type Phase = 'booting' | 'ready' | 'needs-login' | 'blocked' | 'maintenance' | 'error'

interface SessionState {
  phase: Phase
  me: Me | null
  catalog: Catalog | null
  app: AppSettings | null
  error: string
  blockedReason: string
  unread: number
  boot(): Promise<void>
  refresh(): Promise<Me | null>
  setMe(me: Me): void
  devLogin(tgId: number, name: string): Promise<void>
  loadUnread(): Promise<void>
}

async function loginWithTelegram(): Promise<boolean> {
  const initData = platform.initData()
  if (!initData) return false
  const r = await api<{ token: string }>('POST', '/api/auth/telegram', { body: { init_data: initData }, noAuth: true })
  auth.set(r.token)
  return true
}

export const useSession = create<SessionState>((set, getState) => ({
  phase: 'booting',
  me: null,
  catalog: null,
  app: null,
  error: '',
  blockedReason: '',
  unread: 0,

  async boot() {
    setUnauthorizedHandler(async () => {
      try {
        return await loginWithTelegram()
      } catch {
        auth.set(null)
        return false
      }
    })
    try {
      const cat = await get<{ catalog: Catalog; app: AppSettings }>('/api/catalog')
      set({ catalog: cat.catalog, app: cat.app })

      // Inside Telegram we always re-authenticate: signed init data is the source of truth.
      if (platform.inTelegram()) {
        try {
          await loginWithTelegram()
        } catch (e) {
          // Telegram replays old launch data when it restores a minimised Mini App, so it can be
          // past the server's max age. A session we already hold is still good — keep using it.
          if ((e as ApiError).status !== 401 || !auth.get()) throw e
        }
      }
      if (!auth.get()) {
        set({ phase: 'needs-login' })
        return
      }
      const me = await get<Me>('/api/me')
      set({ me, phase: 'ready' })
    } catch (e) {
      const err = e as ApiError
      if (err.code === 'maintenance') return set({ phase: 'maintenance', error: err.message })
      if (err.code?.startsWith('account_')) return set({ phase: 'blocked', blockedReason: err.message, error: String(err.data?.reason ?? '') })
      if (err.status === 401) {
        auth.set(null)
        return set({ phase: 'needs-login' })
      }
      set({ phase: 'error', error: err.message || 'Could not start Atish' })
    }
  },

  async refresh() {
    try {
      const me = await get<Me>('/api/me')
      set({ me })
      return me
    } catch (e) {
      const err = e as ApiError
      if (err.code === 'maintenance') set({ phase: 'maintenance', error: err.message })
      return null
    }
  },

  setMe: (me) => set({ me }),

  async devLogin(tgId, name) {
    const r = await api<{ token: string }>('POST', '/api/auth/dev', { body: { telegram_id: tgId, name }, noAuth: true })
    auth.set(r.token)
    set({ phase: 'booting' })
    const me = await get<Me>('/api/me')
    set({ me, phase: 'ready' })
  },

  async loadUnread() {
    try {
      const r = await get<{ unread: number }>('/api/chats/unread')
      if (r.unread !== getState().unread) set({ unread: r.unread })
    } catch {
      /* ignore */
    }
  },
}))

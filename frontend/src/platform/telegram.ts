/**
 * Platform adapter. The rest of the app only talks to this module, never to
 * window.Telegram directly, so a web / Android / iOS shell can provide its own
 * implementation of the same functions later.
 */
type TgWebApp = {
  initData: string
  initDataUnsafe: {
    user?: { id: number; first_name?: string; last_name?: string; username?: string; language_code?: string; photo_url?: string }
    start_param?: string
  }
  colorScheme: 'light' | 'dark'
  platform: string
  version: string
  ready(): void
  expand(): void
  close(): void
  disableVerticalSwipes?(): void
  setHeaderColor?(c: string): void
  setBackgroundColor?(c: string): void
  setBottomBarColor?(c: string): void
  onEvent(e: string, cb: () => void): void
  offEvent(e: string, cb: () => void): void
  openLink(url: string): void
  openTelegramLink(url: string): void
  openInvoice(url: string, cb?: (status: 'paid' | 'cancelled' | 'failed' | 'pending') => void): void
  requestContact?(cb?: (shared: boolean) => void): void
  HapticFeedback?: { impactOccurred(s: string): void; notificationOccurred(s: string): void; selectionChanged(): void }
  BackButton?: { show(): void; hide(): void; onClick(cb: () => void): void; offClick(cb: () => void): void }
}

const tg = (): TgWebApp | undefined => (window as unknown as { Telegram?: { WebApp?: TgWebApp } }).Telegram?.WebApp

export const platform = {
  /** Raw, signed init data to send to the backend. Empty outside Telegram. */
  initData: (): string => tg()?.initData ?? '',
  inTelegram: (): boolean => !!tg()?.initData,
  prefill: () => tg()?.initDataUnsafe?.user,
  startParam: (): string | undefined => tg()?.initDataUnsafe?.start_param,
  scheme: (): 'light' | 'dark' => {
    const w = tg()
    if (w?.initData) return w.colorScheme
    return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  },

  init() {
    const w = tg()
    if (!w) return
    try {
      w.ready()
      w.expand()
      w.disableVerticalSwipes?.() // keeps card swipes from collapsing the Mini App
    } catch {
      /* older clients */
    }
  },

  onThemeChange(cb: () => void) {
    tg()?.onEvent('themeChanged', cb)
    window.matchMedia?.('(prefers-color-scheme: dark)')?.addEventListener?.('change', cb)
  },

  paint(bg: string) {
    const w = tg()
    try {
      w?.setHeaderColor?.(bg)
      w?.setBackgroundColor?.(bg)
      w?.setBottomBarColor?.(bg)
    } catch {
      /* unsupported */
    }
  },

  haptic(kind: 'light' | 'medium' | 'success' | 'error' | 'select' = 'light') {
    const h = tg()?.HapticFeedback
    if (!h) return
    try {
      if (kind === 'success' || kind === 'error') h.notificationOccurred(kind)
      else if (kind === 'select') h.selectionChanged()
      else h.impactOccurred(kind)
    } catch {
      /* ignore */
    }
  },

  openLink(url: string) {
    const w = tg()
    if (w?.initData) w.openLink(url)
    else window.open(url, '_blank', 'noopener')
  },

  openTelegramLink(url: string) {
    const w = tg()
    if (w?.initData) w.openTelegramLink(url)
    else window.open(url, '_blank', 'noopener')
  },

  openInvoice(url: string): Promise<'paid' | 'cancelled' | 'failed' | 'pending'> {
    return new Promise((resolve) => {
      const w = tg()
      if (!w?.initData) return resolve('failed')
      w.openInvoice(url, resolve)
    })
  },

  canRequestContact: (): boolean => !!tg()?.requestContact && !!tg()?.initData,
  requestContact(): Promise<boolean> {
    return new Promise((resolve) => {
      const w = tg()
      if (!w?.requestContact) return resolve(false)
      w.requestContact((shared) => resolve(!!shared))
    })
  },

  /** Shows Telegram's native back button; returns a cleanup function. */
  showBack(cb: () => void): () => void {
    const b = tg()?.BackButton
    if (!b || !tg()?.initData) return () => {}
    b.onClick(cb)
    b.show()
    return () => {
      b.offClick(cb)
      b.hide()
    }
  },
}

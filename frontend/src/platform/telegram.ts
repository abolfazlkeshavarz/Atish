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

/**
 * Telegram hands the Mini App its signed launch data in the URL hash (#tgWebAppData=...&tgWebAppThemeParams=...).
 * We read it ourselves instead of relying on telegram-web-app.js, because that script is served from telegram.org
 * and does not load on networks where telegram.org is filtered — sign-in must still work there. Stored under the
 * SDK's own sessionStorage key so reloads, in-app navigation and a late-loading SDK all see the same data.
 */
const LAUNCH_KEY = '__telegram__initParams'
function readLaunchParams(): Record<string, string> {
  const params: Record<string, string> = {}
  try {
    Object.assign(params, JSON.parse(sessionStorage.getItem(LAUNCH_KEY) || '{}'))
  } catch {
    /* storage blocked or empty */
  }
  try {
    const hash = location.hash.replace(/^#/, '')
    const query = hash.slice(hash.indexOf('?') + 1)
    for (const part of query.split('&')) {
      const eq = part.indexOf('=')
      if (eq > 0 && part.startsWith('tgWebApp')) params[part.slice(0, eq)] = decodeURIComponent(part.slice(eq + 1))
    }
    if (Object.keys(params).length) sessionStorage.setItem(LAUNCH_KEY, JSON.stringify(params))
  } catch {
    /* malformed hash */
  }
  return params
}
const launch = readLaunchParams()

const rawInitData = (): string => tg()?.initData || launch.tgWebAppData || ''

/** User / start_param from the launch data, for when the SDK isn't loaded. Unverified — display and prefill only. */
function launchField<T>(key: string, json: boolean): T | undefined {
  try {
    const v = new URLSearchParams(launch.tgWebAppData || '').get(key)
    if (v == null) return undefined
    return (json ? JSON.parse(v) : v) as T
  } catch {
    return undefined
  }
}

/** Minimal SDK-less bridge to the Telegram client (same transports telegram-web-app.js uses). */
function postEvent(eventType: string, eventData: unknown = '') {
  const w = window as unknown as { TelegramWebviewProxy?: { postEvent(t: string, d: string): void }; external?: { notify?(m: string): void } }
  try {
    if (w.TelegramWebviewProxy) w.TelegramWebviewProxy.postEvent(eventType, JSON.stringify(eventData))
    else if (w.external?.notify) w.external.notify(JSON.stringify({ eventType, eventData }))
    else if (window.parent !== window) window.parent.postMessage(JSON.stringify({ eventType, eventData }), 'https://web.telegram.org')
  } catch {
    /* not in a Telegram webview */
  }
}

function launchScheme(): 'light' | 'dark' | undefined {
  try {
    const bg = (JSON.parse(launch.tgWebAppThemeParams || '{}') as { bg_color?: string }).bg_color
    const m = bg && /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(bg)
    if (!m) return undefined
    const [r, g, b] = [m[1], m[2], m[3]].map((h) => parseInt(h, 16))
    return 0.299 * r + 0.587 * g + 0.114 * b < 128 ? 'dark' : 'light'
  } catch {
    return undefined
  }
}

export const platform = {
  /** Raw, signed init data to send to the backend. Empty outside Telegram. */
  initData: rawInitData,
  inTelegram: (): boolean => !!rawInitData(),
  prefill: () => tg()?.initDataUnsafe?.user ?? launchField<TgWebApp['initDataUnsafe']['user']>('user', true),
  startParam: (): string | undefined => tg()?.initDataUnsafe?.start_param ?? launchField<string>('start_param', false),
  scheme: (): 'light' | 'dark' => {
    const w = tg()
    if (w?.initData) return w.colorScheme
    const fromLaunch = launchScheme()
    if (fromLaunch) return fromLaunch
    return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  },

  init() {
    const w = tg()
    if (!w) {
      // SDK not loaded (yet): still tell Telegram the page is up and ask for full height
      if (rawInitData()) {
        postEvent('web_app_ready')
        postEvent('web_app_expand')
      }
      return
    }
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

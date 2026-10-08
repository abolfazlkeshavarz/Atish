import { useEffect, useState } from 'react'
import { Check, Crown, Eye, Heart, Undo2, Zap } from 'lucide-react'
import { get, post } from '@/api/client'
import type { Entitlements, Plan } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError, useUI } from '@/store/ui'
import { platform } from '@/platform/telegram'
import { Button, Sheet, Spinner, cx } from '@/components/ui'
import { money } from '@/lib/format'

const PERKS = [
  { icon: <Heart className="h-5 w-5" />, title: 'Unlimited likes', sub: 'Never hit the daily limit' },
  { icon: <Eye className="h-5 w-5" />, title: 'See who likes you', sub: 'Match instantly with a tap' },
  { icon: <Undo2 className="h-5 w-5" />, title: 'Rewind', sub: 'Undo an accidental pass' },
  { icon: <Zap className="h-5 w-5" />, title: 'Plus badge', sub: 'Stand out on your profile' },
]

const PROVIDER_LABEL: Record<string, string> = { telegram_stars: '⭐ Pay with Telegram Stars', stripe: '💳 Pay by card' }

/** Global upgrade sheet, opened from anywhere via useUI().openPremium(). */
export default function PremiumSheet() {
  const reason = useUI((s) => s.premiumPrompt)
  const close = useUI((s) => s.closePremium)
  const { me, refresh } = useSession()
  const [plans, setPlans] = useState<Plan[] | null>(null)
  const [ent, setEnt] = useState<Entitlements | null>(null)
  const [sel, setSel] = useState<string>('')
  const [busy, setBusy] = useState('')

  useEffect(() => {
    if (!reason) return
    get<{ items: Plan[]; entitlements: Entitlements }>('/api/billing/plans')
      .then((r) => { setPlans(r.items); setEnt(r.entitlements); setSel((s) => s || r.items.find((p) => p.interval === 'year')?.code || r.items[0]?.code || '') })
      .catch(toastError)
  }, [reason])

  if (!reason) return null
  const plan = plans?.find((p) => p.code === sel)
  const headline = reason === 'limit' ? "You're out of likes for today" : reason === 'see_likes' ? 'See everyone who likes you' : reason === 'rewind' ? 'Undo your last pass' : 'Get more from Atish'

  const waitForPremium = async () => {
    for (let i = 0; i < 10; i++) {
      const m = await refresh()
      if (m?.entitlements.premium) { toast('Welcome to Atish Plus ✨', 'success'); platform.haptic('success'); close(); return }
      await new Promise((r) => setTimeout(r, 1500))
    }
    toast('Payment received — it may take a moment to activate')
  }

  const pay = async (provider: string) => {
    if (!plan) return
    setBusy(provider)
    try {
      const r = await post<{ url?: string; invoice_link?: string }>('/api/billing/checkout', { plan: plan.code, provider })
      if (r.invoice_link) {
        const status = await platform.openInvoice(r.invoice_link)
        if (status === 'paid') await waitForPremium()
      } else if (r.url) {
        platform.openLink(r.url)
        toast('Complete the payment in your browser, then come back')
        // user returns to the app after paying; poll briefly
        void waitForPremium()
      }
    } catch (e) { toastError(e) } finally { setBusy('') }
  }

  const active = me?.entitlements.premium

  return (
    <Sheet open onClose={close} tall>
      <div className="-mt-1 pb-2 text-center">
        <div className="brand-gradient mx-auto mb-4 flex h-20 w-20 items-center justify-center rounded-3xl shadow-glow"><Crown className="h-10 w-10 text-white" /></div>
        <h2 className="text-[26px] font-extrabold leading-tight">{active ? 'You have Atish Plus' : headline}</h2>
        <p className="mt-1 text-sm text-muted">
          {active ? (ent?.until ? `Active until ${new Date(ent.until).toLocaleDateString()}` : 'Thank you for supporting Atish!') : 'Atish is free forever. Plus is for people who want more.'}
        </p>
      </div>

      <div className="my-5 space-y-3">
        {PERKS.map((p) => (
          <div key={p.title} className="flex items-center gap-3.5 rounded-2xl border border-line/70 bg-surface p-3.5 shadow-soft">
            <span className="brand-gradient flex h-10 w-10 items-center justify-center rounded-xl text-white">{p.icon}</span>
            <div><div className="font-bold">{p.title}</div><div className="text-xs text-muted">{p.sub}</div></div>
            <Check className="ml-auto h-5 w-5 text-ok" />
          </div>
        ))}
      </div>

      {!active && (
        plans === null ? <div className="flex justify-center py-6"><Spinner /></div> : plans.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted">Plus isn't available for purchase right now.</p>
        ) : (
          <>
            <div className="mb-4 grid grid-cols-2 gap-3">
              {plans.map((p) => {
                const monthly = p.interval === 'year' ? p.price_cents / 12 : p.price_cents
                return (
                  <button key={p.code} onClick={() => setSel(p.code)}
                    className={cx('press relative rounded-2xl border-2 p-4 text-left transition', sel === p.code ? 'border-brand bg-brand/8 shadow-glow' : 'border-line bg-surface')}>
                    {p.interval === 'year' && <span className="brand-gradient absolute -top-2.5 right-3 rounded-full px-2 py-0.5 text-[10px] font-extrabold text-white">BEST VALUE</span>}
                    <div className="text-xs font-bold uppercase text-muted">{p.interval === 'year' ? 'Yearly' : p.interval === 'month' ? 'Monthly' : 'Lifetime'}</div>
                    <div className="mt-1 text-xl font-extrabold">{p.price_cents ? money(p.price_cents, p.currency) : `⭐ ${p.stars_price}`}</div>
                    {p.price_cents > 0 && p.interval === 'year' && <div className="text-xs text-muted">{money(monthly, p.currency)}/mo</div>}
                    {p.stars_price ? <div className="text-xs text-muted">or ⭐ {p.stars_price}</div> : null}
                  </button>
                )
              })}
            </div>
            <div className="space-y-2.5 pb-2">
              {plan?.providers.map((pr, i) => (
                <Button key={pr} block size="lg" variant={i === 0 ? 'primary' : 'outline'} loading={busy === pr} onClick={() => pay(pr)}>
                  {PROVIDER_LABEL[pr] ?? `Pay with ${pr}`}
                </Button>
              ))}
              <p className="pt-1 text-center text-[11px] text-faint">Cancel anytime. Payments are handled securely by Telegram / Stripe.</p>
            </div>
          </>
        )
      )}
    </Sheet>
  )
}

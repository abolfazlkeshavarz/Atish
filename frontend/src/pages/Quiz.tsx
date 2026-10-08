import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { patch } from '@/api/client'
import type { Me } from '@/api/types'
import { useSession } from '@/store/session'
import { toast, toastError } from '@/store/ui'
import { platform } from '@/platform/telegram'
import { Button, Progress, cx } from '@/components/ui'

/** One question per screen, auto-advancing — about 30 seconds for 12 questions. */
export default function Quiz() {
  const nav = useNavigate()
  const { me, catalog, setMe } = useSession()
  const qs = catalog?.personality_questions ?? []
  const [i, setI] = useState(0)
  const [answers, setAnswers] = useState<Record<string, string>>(me?.personality ?? {})
  const [busy, setBusy] = useState(false)
  if (!me) return null
  if (qs.length === 0) return <div className="p-8 text-center text-muted">No questions right now.</div>
  const q = qs[Math.min(i, qs.length - 1)]

  const finish = async (final: Record<string, string>) => {
    setBusy(true)
    try {
      setMe(await patch<Me>('/api/me/preferences', { personality: final }))
      platform.haptic('success')
      toast('Nice! Your matches just got smarter ✨', 'success')
      nav('/profile', { replace: true })
    } catch (e) { toastError(e) } finally { setBusy(false) }
  }

  const choose = (key: string) => {
    const next = { ...answers, [q.key]: key }
    setAnswers(next)
    platform.haptic('select')
    if (i < qs.length - 1) setTimeout(() => setI(i + 1), 180)
  }

  const last = i === qs.length - 1

  return (
    <div className="mx-auto flex h-full max-w-xl flex-col px-5" style={{ paddingTop: 'calc(14px + var(--safe-t))' }}>
      <div className="mb-6 flex items-center gap-4">
        <button onClick={() => (i === 0 ? nav(-1) : setI(i - 1))} className="press flex h-10 w-10 items-center justify-center rounded-full bg-elevated" aria-label="Back"><ArrowLeft className="h-5 w-5" /></button>
        <Progress value={((i + 1) / qs.length) * 100} className="flex-1" />
        <span className="text-xs font-semibold text-muted">{i + 1}/{qs.length}</span>
      </div>

      <div key={q.key} className="flex-1 animate-rise">
        <h1 className="mb-6 text-[26px] font-extrabold leading-tight">{q.text}</h1>
        <div className="space-y-3">
          {q.options.map((o) => (
            <button
              key={o.key}
              onClick={() => choose(o.key)}
              className={cx('press w-full rounded-2xl border-2 px-5 py-4 text-left text-base font-semibold transition', answers[q.key] === o.key ? 'border-brand bg-brand/10 text-brand' : 'border-line bg-surface')}
            >
              {o.label}
            </button>
          ))}
        </div>
      </div>

      <div style={{ paddingBottom: 'calc(16px + var(--safe-b))' }} className="pt-3">
        <Button block size="lg" loading={busy} onClick={() => (last ? finish(answers) : setI(i + 1))} disabled={last && !answers[q.key]}>
          {last ? 'Save my answers' : answers[q.key] ? 'Next' : 'Skip'}
        </Button>
        {!last && Object.keys(answers).length > 0 && (
          <button onClick={() => finish(answers)} className="press mt-2 w-full py-2 text-sm font-semibold text-muted">Save & finish later</button>
        )}
      </div>
    </div>
  )
}

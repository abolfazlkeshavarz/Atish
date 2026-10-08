import { create } from 'zustand'

export interface Toast { id: number; text: string; kind: 'info' | 'error' | 'success' }
let n = 0

interface UIState {
  toasts: Toast[]
  toast(text: string, kind?: Toast['kind']): void
  dismiss(id: number): void
  premiumPrompt: string | null
  openPremium(reason?: string): void
  closePremium(): void
}

export const useUI = create<UIState>((set, get) => ({
  toasts: [],
  toast(text, kind = 'info') {
    const id = ++n
    set({ toasts: [...get().toasts, { id, text, kind }] })
    setTimeout(() => get().dismiss(id), 3200)
  },
  dismiss: (id) => set({ toasts: get().toasts.filter((t) => t.id !== id) }),
  premiumPrompt: null,
  openPremium: (reason = '') => set({ premiumPrompt: reason || 'premium' }),
  closePremium: () => set({ premiumPrompt: null }),
}))

export const toast = (text: string, kind: Toast['kind'] = 'info') => useUI.getState().toast(text, kind)
export const toastError = (e: unknown) => toast((e as Error)?.message || 'Something went wrong', 'error')

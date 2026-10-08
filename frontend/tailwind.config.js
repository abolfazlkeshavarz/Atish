/** Design tokens live in CSS variables (src/index.css) so light/dark and Telegram themes swap instantly. */
const c = (v) => `rgb(var(--${v}) / <alpha-value>)`
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: ['class', '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        bg: c('bg'), surface: c('surface'), elevated: c('elevated'), line: c('line'),
        ink: c('ink'), muted: c('muted'), faint: c('faint'),
        brand: c('brand'), brand2: c('brand2'), friend: c('friend'), love: c('love'),
        ok: c('ok'), warn: c('warn'), danger: c('danger'),
      },
      fontFamily: { sans: ['Inter', 'ui-sans-serif', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'] },
      borderRadius: { xl2: '1.25rem', xl3: '1.75rem' },
      boxShadow: {
        card: '0 10px 40px -12px rgb(var(--shadow) / 0.35)',
        soft: '0 2px 14px -4px rgb(var(--shadow) / 0.18)',
        glow: '0 8px 30px -6px rgb(var(--brand) / 0.55)',
      },
      keyframes: {
        pop: { '0%': { transform: 'scale(.85)', opacity: 0 }, '100%': { transform: 'scale(1)', opacity: 1 } },
        rise: { '0%': { transform: 'translateY(12px)', opacity: 0 }, '100%': { transform: 'translateY(0)', opacity: 1 } },
        sheet: { '0%': { transform: 'translateY(100%)' }, '100%': { transform: 'translateY(0)' } },
        fade: { '0%': { opacity: 0 }, '100%': { opacity: 1 } },
        shimmer: { '100%': { transform: 'translateX(100%)' } },
        float: { '0%': { transform: 'translateY(0) scale(.6)', opacity: 0 }, '15%': { opacity: 0.9 }, '100%': { transform: 'translateY(-85vh) scale(1.15)', opacity: 0 } },
        heart: { '0%': { transform: 'scale(.4)', opacity: 0 }, '40%': { transform: 'scale(1.25)', opacity: 1 }, '100%': { transform: 'scale(1)', opacity: 1 } },
      },
      animation: {
        pop: 'pop .25s cubic-bezier(.2,.9,.3,1.3)', rise: 'rise .35s ease-out both', sheet: 'sheet .28s cubic-bezier(.2,.8,.2,1)',
        fade: 'fade .2s ease-out', float: 'float 3.6s ease-out both', heart: 'heart .5s cubic-bezier(.2,.9,.3,1.3) both',
      },
    },
  },
  plugins: [],
}

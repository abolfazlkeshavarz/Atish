import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { platform } from './platform/telegram'
import './index.css'

function applyTheme() {
  const scheme = platform.scheme()
  document.documentElement.dataset.theme = scheme
  // paint the Telegram chrome with the app background
  requestAnimationFrame(() => platform.paint(getComputedStyle(document.body).backgroundColor ? scheme === 'dark' ? '#0f0f14' : '#f9f9fb' : '#f9f9fb'))
}

platform.init()
applyTheme()
platform.onThemeChange(applyTheme)

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </React.StrictMode>,
)

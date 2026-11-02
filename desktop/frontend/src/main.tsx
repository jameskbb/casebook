import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
// Fonts are bundled (D-13); nothing is loaded from a network.
import '@fontsource/public-sans/400.css'
import '@fontsource/public-sans/600.css'
import '@fontsource/literata/400.css'
import './styles/tokens.css'
import './styles/base.css'
import { startTheme } from './lib/windowTheme'
import App from './App'

startTheme()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

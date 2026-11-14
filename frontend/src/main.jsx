import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import '@fontsource/poppins/400.css'
import '@fontsource/poppins/500.css'
import '@fontsource/poppins/600.css'
import '@fontsource/ibm-plex-mono/400.css'
import './theme.css'
import './styles.css'
import './overview.css'
import './presets.css'
import './theme-system.css'
import { App } from './App.jsx'
import { AuthProvider, useAuth } from './state/AuthContext.jsx'
import { AppearanceProvider } from './state/AppearanceContext.jsx'

function AccountAppearance() {
  const { user } = useAuth()
  return <AppearanceProvider key={user?.id || 'guest'} accountId={user?.id} loadAccount={Boolean(user)}><App /></AppearanceProvider>
}

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <AccountAppearance />
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>,
)

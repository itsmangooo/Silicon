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
import { App } from './App.jsx'
import { AuthProvider } from './state/AuthContext.jsx'
import { AppearanceProvider } from './state/AppearanceContext.jsx'

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <BrowserRouter>
      <AppearanceProvider>
        <AuthProvider>
          <App />
        </AuthProvider>
      </AppearanceProvider>
    </BrowserRouter>
  </React.StrictMode>,
)

import { StrictMode, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { ThemeProvider } from './contexts/ThemeContext'
import { BrandingProvider } from './contexts/BrandingContext'
import { AuthProvider } from './context/AuthContext'
import { OperationsProvider } from './contexts/OperationsContext'
import { router } from './router'
import './i18n'
import '@fortawesome/fontawesome-free/css/all.min.css'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import './index.css'
import './theme.css'
import './App.css'
import './nexusai-elite.css'
import NexusLoadingState from './components/NexusLoadingState'
import { investigationWorkspaceIdentity } from './analyst/investigationWorkspace'

function BootFallback() {
  if (window.location.pathname.startsWith('/analyst')) {
    return <NexusLoadingState label="Opening investigation workspace…" brandProps={{ instanceName: investigationWorkspaceIdentity.name, instanceTagline: '' }} staticBrand fullScreen />
  }
  return <NexusLoadingState label="Starting NexusAI…" staticBrand fullScreen />
}

// BrandingProvider sits outside AuthProvider so the login screen — which
// renders before authentication completes — can pick up the configured
// instance name and logo from the public /api/branding endpoint.
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Suspense fallback={<BootFallback />}>
      <ThemeProvider>
        <BrandingProvider>
          <AuthProvider>
            <OperationsProvider>
              <RouterProvider router={router} />
            </OperationsProvider>
          </AuthProvider>
        </BrandingProvider>
      </ThemeProvider>
    </Suspense>
  </StrictMode>,
)

import React from 'react'
import ReactDOM from 'react-dom/client'
import '@fontsource-variable/noto-sans/wght.css'
import '@fontsource-variable/noto-sans-arabic/wght.css'
import '@fontsource-variable/noto-sans-mono/wght.css'
import { WorkspaceRouter } from './router.jsx'
// The old design, in the lowest cascade layer, for pages that have not been rebuilt yet; then the new design layers on top of it.
import './styles/legacy.css'
import './v2/styles/index.css'

document.documentElement.classList.add('analyst-portal')
document.getElementById('root').classList.add('analyst-portal__root')

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <WorkspaceRouter />
  </React.StrictMode>,
)

import React from 'react'
import ReactDOM from 'react-dom/client'
import '@fontsource-variable/noto-sans/wght.css'
import '@fontsource-variable/noto-sans-arabic/wght.css'
import '@fontsource-variable/noto-sans-mono/wght.css'
import { WorkspaceRouter } from './router.jsx'
import './styles/tokens.css'
import './styles/workspace.css'
import './styles/identity-a.css'
import './styles/shell.css'
import './styles/canvas.css'

document.documentElement.classList.add('analyst-portal')
document.getElementById('root').classList.add('analyst-portal__root')

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <WorkspaceRouter />
  </React.StrictMode>,
)

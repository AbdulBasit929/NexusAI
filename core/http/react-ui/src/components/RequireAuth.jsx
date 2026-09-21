import { Navigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import NexusLoadingState from './NexusLoadingState'

export default function RequireAuth({ children }) {
  const { authEnabled, staticApiKeyRequired, user, loading } = useAuth()
  if (loading) return <NexusLoadingState label="Verifying secure access…" fullScreen />
  if ((authEnabled || staticApiKeyRequired) && !user) return <Navigate to="/login" replace />
  return children
}

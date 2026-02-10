import React, { useEffect, useState } from 'react'
import { Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom'
import Login from '@/pages/Login'
import CyberDashboard from '@/pages/CyberDashboard'
import NetworkDetail from '@/pages/NetworkDetail'
import AppLayout from '@/components/Layout'

const App: React.FC = () => {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    const token = localStorage.getItem('token')
    const hasToken = !!token
    console.log('App mounted, token exists:', hasToken)
    setIsAuthenticated(hasToken)

    // Redirect if authenticated and on login page
    if (hasToken && location.pathname === '/login') {
      navigate('/dashboard', { replace: true })
    }
  }, [location.pathname])

  const handleLoginSuccess = () => {
    console.log('Login success callback')
    setIsAuthenticated(true)
    navigate('/dashboard', { replace: true })
  }

  const handleLogout = () => {
    localStorage.removeItem('token')
    setIsAuthenticated(false)
    navigate('/login', { replace: true })
  }

  // Show login if not authenticated
  if (!isAuthenticated) {
    return (
      <Routes>
        <Route path="/login" element={<Login onLoginSuccess={handleLoginSuccess} />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  // Show main app if authenticated
  return (
    <AppLayout onLogout={handleLogout}>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/login" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<CyberDashboard />} />
        <Route path="/networks/:id" element={<NetworkDetail />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </AppLayout>
  )
}

export default App

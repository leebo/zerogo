import React, { useEffect, useState } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { Layout } from 'antd'
import Login from '@/pages/Login'
import Dashboard from '@/pages/Dashboard'
import NetworkDetail from '@/pages/NetworkDetail'
import AppLayout from '@/components/Layout'

const { Content } = Layout

const App: React.FC = () => {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const location = useLocation()

  useEffect(() => {
    const token = localStorage.getItem('token')
    setIsAuthenticated(!!token)
  }, [])

  if (!isAuthenticated && location.pathname !== '/login') {
    return <Login onLoginSuccess={() => setIsAuthenticated(true)} />
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      {isAuthenticated ? (
        <AppLayout onLogout={() => setIsAuthenticated(false)}>
          <Content style={{ padding: '24px' }}>
            <Routes>
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route path="/login" element={<Navigate to="/dashboard" replace />} />
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/networks/:id" element={<NetworkDetail />} />
              <Route path="*" element={<Navigate to="/dashboard" replace />} />
            </Routes>
          </Content>
        </AppLayout>
      ) : (
        <Routes>
          <Route path="/login" element={<Login onLoginSuccess={() => setIsAuthenticated(true)} />} />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      )}
    </Layout>
  )
}

export default App

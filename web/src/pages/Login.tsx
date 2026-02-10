import React, { useState } from 'react'
import { motion } from 'framer-motion'
import { Shield, Lock, User } from 'lucide-react'
import { Form, Input, Button } from 'antd'
import { authApi } from '@/api'
import { cyberMessage } from '@/components/ui/CyberMessage'
import type { LoginRequest } from '@/types'

interface LoginProps {
  onLoginSuccess: () => void
}

const Login: React.FC<LoginProps> = ({ onLoginSuccess }) => {
  const [loading, setLoading] = useState(false)
  const isMobile = window.innerWidth < 768

  const onFinish = async (values: LoginRequest) => {
    setLoading(true)
    try {
      const response = await authApi.login(values)
      localStorage.setItem('token', response.token)
      cyberMessage.success('Login successful')
      onLoginSuccess()
    } catch (error) {
      // Error already handled by axios interceptor
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
        overflow: 'hidden',
        background: '#0a0e27',
      }}
    >
      {/* Animated background effects */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: `
            radial-gradient(circle at 20% 50%, rgba(124, 58, 237, 0.15) 0%, transparent 50%),
            radial-gradient(circle at 80% 80%, rgba(0, 240, 255, 0.15) 0%, transparent 50%),
            radial-gradient(circle at 40% 20%, rgba(16, 185, 129, 0.1) 0%, transparent 50%)
          `,
          animation: 'gradientShift 15s ease infinite',
        }}
      />

      {/* Grid overlay */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundImage: `
            linear-gradient(rgba(148, 163, 184, 0.03) 1px, transparent 1px),
            linear-gradient(90deg, rgba(148, 163, 184, 0.03) 1px, transparent 1px)
          `,
          backgroundSize: '50px 50px',
        }}
      />

      {/* Floating orbs */}
      <motion.div
        animate={{
          y: [0, -30, 0],
          opacity: [0.3, 0.6, 0.3],
        }}
        transition={{
          duration: 8,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
        style={{
          position: 'absolute',
          width: '300px',
          height: '300px',
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(0, 240, 255, 0.2) 0%, transparent 70%)',
          filter: 'blur(40px)',
          top: '10%',
          left: '20%',
        }}
      />
      <motion.div
        animate={{
          y: [0, 30, 0],
          opacity: [0.3, 0.6, 0.3],
        }}
        transition={{
          duration: 10,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
        style={{
          position: 'absolute',
          width: '400px',
          height: '400px',
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(124, 58, 237, 0.2) 0%, transparent 70%)',
          filter: 'blur(50px)',
          bottom: '10%',
          right: '15%',
        }}
      />

      {/* Login card */}
      <motion.div
        initial={{ opacity: 0, y: 20, scale: 0.95 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.6, ease: 'easeOut' }}
        style={{
          position: 'relative',
          zIndex: 1,
          width: '100%',
          maxWidth: '440px',
          padding: '2rem',
        }}
      >
        <div
          className="cyber-card"
          style={{
            padding: isMobile ? '2rem 1.5rem' : '3rem',
            background: 'linear-gradient(135deg, rgba(26, 34, 64, 0.9) 0%, rgba(17, 24, 52, 0.95) 100%)',
            border: '1px solid rgba(0, 240, 255, 0.2)',
            borderRadius: '24px',
            boxShadow: '0 20px 60px rgba(0, 0, 0, 0.5)',
          }}
        >
          {/* Logo */}
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            transition={{ delay: 0.2, type: 'spring', stiffness: 200 }}
            style={{
              width: isMobile ? '64px' : '80px',
              height: isMobile ? '64px' : '80px',
              margin: '0 auto 2rem',
              borderRadius: '20px',
              background: 'linear-gradient(135deg, rgba(0, 240, 255, 0.2) 0%, rgba(124, 58, 237, 0.2) 100%)',
              border: '2px solid rgba(0, 240, 255, 0.3)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Shield size={isMobile ? 32 : 40} style={{ color: '#00f0ff' }} />
          </motion.div>

          {/* Title */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3 }}
            style={{ textAlign: 'center', marginBottom: '2rem' }}
          >
            <h1
              style={{
                fontFamily: 'Orbitron, sans-serif',
                fontSize: isMobile ? '1.5rem' : '2rem',
                fontWeight: 600,
                marginBottom: '0.5rem',
                background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              ZeroGo
            </h1>
            <p style={{ color: '#94a3b8', fontSize: isMobile ? '0.75rem' : '0.875rem' }}>
              Secure P2P VPN Mesh Network
            </p>
          </motion.div>

          {/* Form */}
          <Form
            name="login"
            onFinish={onFinish}
            autoComplete="off"
            layout="vertical"
          >
            <motion.div
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 0.4 }}
            >
              <Form.Item
                name="username"
                rules={[{ required: true, message: 'Please input your username!' }]}
              >
                <Input
                  prefix={<User size={18} style={{ color: '#64748b' }} />}
                  placeholder="Username"
                  size="large"
                  style={{
                    background: 'rgba(10, 14, 39, 0.5)',
                    border: '1px solid rgba(148, 163, 184, 0.2)',
                    color: '#f1f5f9',
                    borderRadius: '12px',
                    height: '48px',
                  }}
                />
              </Form.Item>
            </motion.div>

            <motion.div
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 0.5 }}
            >
              <Form.Item
                name="password"
                rules={[{ required: true, message: 'Please input your password!' }]}
              >
                <Input.Password
                  prefix={<Lock size={18} style={{ color: '#64748b' }} />}
                  placeholder="Password"
                  size="large"
                  style={{
                    background: 'rgba(10, 14, 39, 0.5)',
                    border: '1px solid rgba(148, 163, 184, 0.2)',
                    color: '#f1f5f9',
                    borderRadius: '12px',
                    height: '48px',
                  }}
                />
              </Form.Item>
            </motion.div>

            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.6 }}
            >
              <Form.Item style={{ marginBottom: 0 }}>
                <Button
                  type="primary"
                  htmlType="submit"
                  block
                  size="large"
                  loading={loading}
                  style={{
                    height: '48px',
                    background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
                    border: 'none',
                    borderRadius: '12px',
                    fontSize: '16px',
                    fontWeight: 600,
                    fontFamily: 'Orbitron, sans-serif',
                    letterSpacing: '0.05em',
                    position: 'relative',
                    overflow: 'hidden',
                  }}
                >
                  <span>Login</span>
                </Button>
              </Form.Item>
            </motion.div>
          </Form>

          {/* Footer */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.7 }}
            style={{
              marginTop: '2rem',
              textAlign: 'center',
              fontSize: '0.75rem',
              color: '#64748b',
            }}
          >
            <p>Default: admin / change-on-first-login</p>
          </motion.div>
        </div>
      </motion.div>

      {/* Animations */}
      <style>{`
        @keyframes gradientShift {
          0%, 100% {
            opacity: 1;
          }
          50% {
            opacity: 0.8;
          }
        }
      `}</style>
    </div>
  )
}

export default Login

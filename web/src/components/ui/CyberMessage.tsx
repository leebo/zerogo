import React, { useEffect, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { CheckCircle, XCircle, AlertCircle, Info, X } from 'lucide-react'

export type CyberMessageType = 'success' | 'error' | 'warning' | 'info'

interface CyberMessageProps {
  type: CyberMessageType
  message: string
  duration?: number
  onClose?: () => void
}

let messageCounter = 0
const messageQueue: Array<{ id: number; type: CyberMessageType; message: string }> = []
let setMessageCallback: ((messages: typeof messageQueue) => void) | null = null

export const showCyberMessage = (type: CyberMessageType, message: string, duration = 3000) => {
  const id = ++messageCounter
  messageQueue.push({ id, type, message })
  setMessageCallback?.([...messageQueue])

  setTimeout(() => {
    const index = messageQueue.findIndex(m => m.id === id)
    if (index > -1) {
      messageQueue.splice(index, 1)
      setMessageCallback?.([...messageQueue])
    }
  }, duration)
}

export const cyberMessage = {
  success: (message: string, duration?: number) => showCyberMessage('success', message, duration),
  error: (message: string, duration?: number) => showCyberMessage('error', message, duration),
  warning: (message: string, duration?: number) => showCyberMessage('warning', message, duration),
  info: (message: string, duration?: number) => showCyberMessage('info', message, duration),
}

const CyberMessageProvider: React.FC = () => {
  const [messages, setMessages] = useState<typeof messageQueue>([])

  useEffect(() => {
    setMessageCallback = setMessages
    return () => {
      setMessageCallback = null
    }
  }, [])

  const typeConfig = {
    success: {
      icon: CheckCircle,
      color: '#10b981',
      bg: 'rgba(16, 185, 129, 0.15)',
      border: 'rgba(16, 185, 129, 0.3)',
    },
    error: {
      icon: XCircle,
      color: '#ef4444',
      bg: 'rgba(239, 68, 68, 0.15)',
      border: 'rgba(239, 68, 68, 0.3)',
    },
    warning: {
      icon: AlertCircle,
      color: '#f59e0b',
      bg: 'rgba(245, 158, 11, 0.15)',
      border: 'rgba(245, 158, 11, 0.3)',
    },
    info: {
      icon: Info,
      color: '#00f0ff',
      bg: 'rgba(0, 240, 255, 0.15)',
      border: 'rgba(0, 240, 255, 0.3)',
    },
  }

  const isMobile = window.innerWidth < 768

  return (
    <div
      style={{
        position: 'fixed',
        top: isMobile ? '16px' : '24px',
        right: isMobile ? '16px' : '24px',
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        gap: '12px',
        pointerEvents: 'none',
      }}
    >
      <AnimatePresence>
        {messages.map((msg) => {
          const config = typeConfig[msg.type]
          const Icon = config.icon

          return (
            <motion.div
              key={msg.id}
              initial={{ opacity: 0, x: 100, scale: 0.9 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: 100, scale: 0.9 }}
              transition={{ duration: 0.3 }}
              style={{
                pointerEvents: 'auto',
                minWidth: isMobile ? '280px' : '360px',
                maxWidth: isMobile ? 'calc(100vw - 32px)' : '420px',
                background: `linear-gradient(135deg, rgba(26, 34, 64, 0.95) 0%, rgba(17, 24, 52, 0.98) 100%)`,
                border: `1px solid ${config.border}`,
                borderRadius: '12px',
                padding: isMobile ? '12px 16px' : '16px',
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
                position: 'relative',
                overflow: 'hidden',
              }}
            >
              {/* Scan line effect */}
              <div
                style={{
                  position: 'absolute',
                  top: 0,
                  left: '-100%',
                  width: '50%',
                  height: '100%',
                  background: `linear-gradient(90deg, transparent, ${config.bg}, transparent)`,
                  animation: 'messageScan 2s linear infinite',
                  pointerEvents: 'none',
                }}
              />

              {/* Icon */}
              <div
                style={{
                  width: '32px',
                  height: '32px',
                  borderRadius: '8px',
                  background: config.bg,
                  border: `1px solid ${config.border}`,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  flexShrink: 0,
                }}
              >
                <Icon size={18} style={{ color: config.color }} />
              </div>

              {/* Message */}
              <span
                style={{
                  flex: 1,
                  color: '#f1f5f9',
                  fontSize: isMobile ? '0.875rem' : '0.9375rem',
                  lineHeight: 1.5,
                }}
              >
                {msg.message}
              </span>

              {/* Close button */}
              <button
                onClick={() => {
                  const index = messageQueue.findIndex(m => m.id === msg.id)
                  if (index > -1) {
                    messageQueue.splice(index, 1)
                    setMessages([...messageQueue])
                  }
                }}
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: '#64748b',
                  cursor: 'pointer',
                  padding: '4px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: '4px',
                  transition: 'all 0.2s',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = 'rgba(148, 163, 184, 0.1)'
                  e.currentTarget.style.color = '#94a3b8'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = 'transparent'
                  e.currentTarget.style.color = '#64748b'
                }}
              >
                <X size={16} />
              </button>

              <style>{`
                @keyframes messageScan {
                  0% { left: -50%; }
                  100% { left: 150%; }
                }
              `}</style>
            </motion.div>
          )
        })}
      </AnimatePresence>
    </div>
  )
}

export default CyberMessageProvider

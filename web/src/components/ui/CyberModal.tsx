import React, { ReactNode } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X } from 'lucide-react'

interface CyberModalProps {
  visible: boolean
  onClose: () => void
  title?: string
  children: ReactNode
}

const CyberModal: React.FC<CyberModalProps> = ({
  visible,
  onClose,
  title,
  children,
}) => {
  const isMobile = window.innerWidth < 768

  return (
    <AnimatePresence>
      {visible && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.3 }}
            onClick={onClose}
            style={{
              position: 'fixed',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              background: 'rgba(10, 14, 39, 0.8)',
              backdropFilter: 'blur(8px)',
              zIndex: 1000,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '2rem',
            }}
          >
            {/* Modal */}
            <motion.div
              initial={{ opacity: 0, scale: 0.9, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.9, y: 20 }}
              transition={{ duration: 0.3, ease: 'easeOut' }}
              onClick={(e) => e.stopPropagation()}
              style={{
                width: '100%',
                maxWidth: '520px',
                background: 'linear-gradient(135deg, rgba(26, 34, 64, 0.95) 0%, rgba(17, 24, 52, 0.98) 100%)',
                border: '1px solid rgba(0, 240, 255, 0.2)',
                borderRadius: '24px',
                boxShadow: '0 20px 60px rgba(0, 0, 0, 0.6), 0 0 40px rgba(0, 240, 255, 0.1)',
                position: 'relative',
                overflow: 'hidden',
              }}
            >
              {/* Corner accents */}
              <div
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '60px',
                  height: '60px',
                  borderTop: '2px solid rgba(0, 240, 255, 0.5)',
                  borderLeft: '2px solid rgba(0, 240, 255, 0.5)',
                  borderTopLeftRadius: '24px',
                }}
              />
              <div
                style={{
                  position: 'absolute',
                  bottom: 0,
                  right: 0,
                  width: '60px',
                  height: '60px',
                  borderBottom: '2px solid rgba(124, 58, 237, 0.5)',
                  borderRight: '2px solid rgba(124, 58, 237, 0.5)',
                  borderBottomRightRadius: '24px',
                }}
              />

              {/* Scan line effect */}
              <div
                style={{
                  position: 'absolute',
                  top: 0,
                  left: '-100%',
                  width: '50%',
                  height: '100%',
                  background: 'linear-gradient(90deg, transparent, rgba(0, 240, 255, 0.05), transparent)',
                  animation: 'scan 4s linear infinite',
                  pointerEvents: 'none',
                }}
              />

              {/* Header */}
              {title && (
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: isMobile ? '1.5rem 1.5rem 1rem' : '2rem 2rem 1.5rem',
                    borderBottom: '1px solid rgba(148, 163, 184, 0.1)',
                  }}
                >
                  <h2
                    style={{
                      fontFamily: 'Orbitron, sans-serif',
                      fontSize: isMobile ? '1.25rem' : '1.5rem',
                      fontWeight: 600,
                      margin: 0,
                      background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
                      WebkitBackgroundClip: 'text',
                      WebkitTextFillColor: 'transparent',
                      backgroundClip: 'text',
                    }}
                  >
                    {title}
                  </h2>
                  <motion.button
                    whileHover={{ scale: 1.1, rotate: 90 }}
                    whileTap={{ scale: 0.9 }}
                    onClick={onClose}
                    style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '8px',
                      background: 'rgba(148, 163, 184, 0.1)',
                      border: '1px solid rgba(148, 163, 184, 0.2)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      cursor: 'pointer',
                      transition: 'all 0.2s',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.background = 'rgba(239, 68, 68, 0.2)'
                      e.currentTarget.style.borderColor = 'rgba(239, 68, 68, 0.4)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = 'rgba(148, 163, 184, 0.1)'
                      e.currentTarget.style.borderColor = 'rgba(148, 163, 184, 0.2)'
                    }}
                  >
                    <X size={18} style={{ color: '#94a3b8' }} />
                  </motion.button>
                </div>
              )}

              {/* Content */}
              <div style={{ padding: title ? (isMobile ? '1.5rem' : '2rem') : (isMobile ? '1.5rem' : '2rem') }}>
                {children}
              </div>

              <style>{`
                @keyframes scan {
                  0% { left: -50%; }
                  100% { left: 150%; }
                }
              `}</style>
            </motion.div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  )
}

export default CyberModal

import React from 'react'
import { motion } from 'framer-motion'

interface CyberCardProps {
  children: React.ReactNode
  className?: string
  delay?: number
  onClick?: () => void
}

const CyberCard: React.FC<CyberCardProps> = ({
  children,
  className = '',
  delay = 0,
  onClick,
}) => {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.6, delay }}
      whileHover={{ y: -4, transition: { duration: 0.2 } }}
      onClick={onClick}
      className={`cyber-card ${onClick ? 'cursor-pointer' : ''} ${className}`}
      style={{
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
          width: '40px',
          height: '40px',
          borderTop: '2px solid var(--color-primary)',
          borderLeft: '2px solid var(--color-primary)',
          borderTopLeftRadius: '16px',
          opacity: 0.5,
        }}
      />
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          right: 0,
          width: '40px',
          height: '40px',
          borderBottom: '2px solid var(--color-secondary)',
          borderRight: '2px solid var(--color-secondary)',
          borderBottomRightRadius: '16px',
          opacity: 0.5,
        }}
      />

      {/* Scan line effect */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: -100,
          width: '50%',
          height: '100%',
          background: 'linear-gradient(90deg, transparent, rgba(0, 240, 255, 0.05), transparent)',
          animation: 'scan 4s linear infinite',
        }}
      />

      <style>{`
        @keyframes scan {
          0% { left: -50%; }
          100% { left: 150%; }
        }
      `}</style>

      {children}
    </motion.div>
  )
}

export default CyberCard

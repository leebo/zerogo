import React from 'react'
import { motion } from 'framer-motion'
import { Network, Users, Activity, ArrowRight } from 'lucide-react'
import CyberCard from './CyberCard'
import StatusIndicator from './StatusIndicator'
import type { Network as NetworkType } from '@/types'

interface NetworkCardProps {
  network: NetworkType
  onClick: () => void
  delay?: number
}

const isMobile = () => window.innerWidth < 768

const NetworkCard: React.FC<NetworkCardProps> = ({ network, onClick, delay = 0 }) => {
  const onlinePercentage = network.member_count > 0
    ? Math.round((network.online_count / network.member_count) * 100)
    : 0

  return (
    <CyberCard
      delay={delay}
      onClick={onClick}
      style={{
        cursor: 'pointer',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* Card Content */}
      <div style={{ padding: isMobile() ? '1.25rem' : '1.5rem' }}>
        {/* Header */}
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          marginBottom: '1rem',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.875rem' }}>
            <div
              style={{
                width: isMobile() ? '44px' : '52px',
                height: isMobile() ? '44px' : '52px',
                borderRadius: '14px',
                background: 'linear-gradient(135deg, rgba(0, 240, 255, 0.2) 0%, rgba(124, 58, 237, 0.2) 100%)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid rgba(0, 240, 255, 0.3)',
                flexShrink: 0,
              }}
            >
              <Network size={isMobile() ? 22 : 26} style={{ color: '#00f0ff' }} />
            </div>

            <div>
              <h3
                style={{
                  fontFamily: 'Orbitron, sans-serif',
                  fontSize: isMobile() ? '1.125rem' : '1.25rem',
                  fontWeight: 600,
                  marginBottom: '0.25rem',
                  color: '#f1f5f9',
                  lineHeight: 1.3,
                }}
              >
                {network.name}
              </h3>
              <p
                style={{
                  fontFamily: 'JetBrains Mono, monospace',
                  fontSize: '0.75rem',
                  color: '#64748b',
                  letterSpacing: '0.025em',
                }}
              >
                ID: {network.id}
              </p>
            </div>
          </div>

          <motion.div
            whileHover={{ scale: 1.1, rotate: 45 }}
            transition={{ duration: 0.2 }}
            style={{ flexShrink: 0 }}
          >
            <ArrowRight size={20} style={{ color: '#00f0ff' }} />
          </motion.div>
        </div>

        {/* Description */}
        {network.description && (
          <p style={{
            fontSize: '0.875rem',
            color: '#94a3b8',
            marginBottom: '1rem',
            lineHeight: 1.5,
            display: '-webkit-box',
            WebkitLineClamp: 2,
            WebkitBoxOrient: 'vertical',
            overflow: 'hidden',
          }}>
            {network.description}
          </p>
        )}

        {/* Stats Row */}
        <div style={{
          display: 'flex',
          gap: '1.5rem',
          marginBottom: '1rem',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Users size={16} style={{ color: '#64748b', flexShrink: 0 }} />
            <div>
              <p style={{
                fontSize: '0.75rem',
                color: '#64748b',
                marginBottom: '0.125rem',
                lineHeight: 1,
              }}>
                Members
              </p>
              <p style={{
                fontSize: '1rem',
                color: '#f1f5f9',
                fontWeight: 500,
                fontFamily: 'Orbitron, sans-serif',
              }}>
                {network.member_count}
              </p>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Activity size={16} style={{ color: '#10b981', flexShrink: 0 }} />
            <div>
              <p style={{
                fontSize: '0.75rem',
                color: '#64748b',
                marginBottom: '0.125rem',
                lineHeight: 1,
              }}>
                Online
              </p>
              <p style={{
                fontSize: '1rem',
                color: '#10b981',
                fontWeight: 500,
                fontFamily: 'Orbitron, sans-serif',
              }}>
                {network.online_count}
              </p>
            </div>
          </div>
        </div>

        {/* Progress Section */}
        <div>
          <div style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: '0.5rem',
          }}>
            <span style={{ fontSize: '0.75rem', color: '#64748b' }}>
              Network Status
            </span>
            <span
              style={{
                fontFamily: 'JetBrains Mono, monospace',
                fontSize: '0.875rem',
                color: '#00f0ff',
                fontWeight: 500,
              }}
            >
              {onlinePercentage}%
            </span>
          </div>

          {/* Progress Bar */}
          <div
            style={{
              width: '100%',
              height: '6px',
              background: 'rgba(26, 34, 64, 0.6)',
              borderRadius: '3px',
              overflow: 'hidden',
            }}
          >
            <motion.div
              initial={{ width: 0 }}
              animate={{ width: `${onlinePercentage}%` }}
              transition={{ duration: 1, delay: delay + 0.3 }}
              style={{
                height: '100%',
                background: 'linear-gradient(90deg, #10b981 0%, #00f0ff 100%)',
                borderRadius: '3px',
                boxShadow: '0 0 10px rgba(0, 240, 255, 0.3)',
              }}
            />
          </div>
        </div>

        {/* Footer */}
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginTop: '1rem',
            paddingTop: '1rem',
            borderTop: '1px solid rgba(148, 163, 184, 0.1)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <StatusIndicator online={network.online_count > 0} size="sm" />
            <span
              style={{
                fontFamily: 'JetBrains Mono, monospace',
                fontSize: '0.75rem',
                color: '#94a3b8',
              }}
            >
              {network.ip_range}
            </span>
          </div>

          <div
            style={{
              padding: '0.25rem 0.75rem',
              borderRadius: '6px',
              background: onlinePercentage > 75
                ? 'rgba(16, 185, 129, 0.2)'
                : onlinePercentage > 25
                ? 'rgba(245, 158, 11, 0.2)'
                : 'rgba(239, 68, 68, 0.2)',
            }}
          >
            <span
              style={{
                fontSize: '0.75rem',
                fontWeight: 600,
                fontFamily: 'Orbitron, sans-serif',
                color: onlinePercentage > 75
                  ? '#10b981'
                  : onlinePercentage > 25
                  ? '#f59e0b'
                  : '#ef4444',
              }}
            >
              {onlinePercentage > 75 ? 'Healthy' : onlinePercentage > 25 ? 'Warning' : 'Critical'}
            </span>
          </div>
        </div>
      </div>
    </CyberCard>
  )
}

export default NetworkCard

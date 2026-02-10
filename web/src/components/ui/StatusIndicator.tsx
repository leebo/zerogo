import React from 'react'

interface StatusIndicatorProps {
  online: boolean
  size?: 'sm' | 'md' | 'lg'
  showText?: boolean
  text?: string
}

const StatusIndicator: React.FC<StatusIndicatorProps> = ({
  online,
  size = 'md',
  showText = false,
  text,
}) => {
  const sizeMap = {
    sm: { width: 6, height: 6 },
    md: { width: 8, height: 8 },
    lg: { width: 12, height: 12 },
  }

  const statusText = text || (online ? 'Online' : 'Offline')

  return (
    <div className="flex items-center gap-2">
      <div
        className="status-indicator"
        style={{
          width: sizeMap[size].width,
          height: sizeMap[size].height,
          backgroundColor: online ? 'var(--color-success)' : 'var(--color-text-muted)',
          boxShadow: online ? '0 0 10px var(--color-success)' : 'none',
        }}
      />
      {showText && (
        <span className="text-sm text-secondary font-mono">{statusText}</span>
      )}
    </div>
  )
}

export default StatusIndicator

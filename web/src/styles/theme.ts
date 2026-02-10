// Cyber Industrial Theme
export const theme = {
  colors: {
    // Deep space backgrounds
    background: '#0a0e27',
    backgroundSecondary: '#111834',
    backgroundElevated: '#1a2240',

    // Accent colors
    primary: '#00f0ff',
    primaryDim: 'rgba(0, 240, 255, 0.1)',
    secondary: '#7c3aed',
    success: '#10b981',
    warning: '#f59e0b',
    error: '#ef4444',

    // Text
    textPrimary: '#f1f5f9',
    textSecondary: '#94a3b8',
    textMuted: '#64748b',

    // Borders
    border: 'rgba(148, 163, 184, 0.1)',
    borderHighlight: 'rgba(0, 240, 255, 0.3)',
  },

  gradients: {
    primary: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
    success: 'linear-gradient(135deg, #10b981 0%, #059669 100%)',
    background: 'linear-gradient(180deg, #0a0e27 0%, #111834 100%)',
    card: 'linear-gradient(135deg, rgba(26, 34, 64, 0.8) 0%, rgba(17, 24, 52, 0.9) 100%)',
  },

  shadows: {
    glow: '0 0 20px rgba(0, 240, 255, 0.3)',
    glowStrong: '0 0 40px rgba(0, 240, 255, 0.5)',
    card: '0 8px 32px rgba(0, 0, 0, 0.4)',
  },

  typography: {
    fontFamily: {
      mono: "'JetBrains Mono', 'Fira Code', monospace",
      sans: "'Space Grotesk', 'Inter', system-ui, sans-serif",
      display: "'Orbitron', 'Rajdhani', sans-serif",
    },
    fontSize: {
      xs: '0.75rem',
      sm: '0.875rem',
      base: '1rem',
      lg: '1.125rem',
      xl: '1.25rem',
      '2xl': '1.5rem',
      '3xl': '2rem',
      '4xl': '2.5rem',
    },
  },

  spacing: {
    xs: '0.5rem',
    sm: '1rem',
    md: '1.5rem',
    lg: '2rem',
    xl: '3rem',
    '2xl': '4rem',
  },

  borderRadius: {
    sm: '4px',
    md: '8px',
    lg: '16px',
    xl: '24px',
    full: '9999px',
  },
}

export default theme

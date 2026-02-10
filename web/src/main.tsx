import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import CyberMessageProvider from '@/components/ui/CyberMessage'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ConfigProvider
        locale={zhCN}
        theme={{
          algorithm: theme.darkAlgorithm,
          token: {
            colorPrimary: '#00f0ff',
            borderRadius: 8,
            colorBgContainer: '#1a2240',
            colorBgElevated: '#1a2240',
            colorBgLayout: '#0a0e27',
            colorBorder: 'rgba(148, 163, 184, 0.1)',
          },
          components: {
            Modal: {
              contentBg: 'rgba(26, 34, 64, 0.95)',
              headerBg: 'transparent',
            },
          },
        }}
      >
        <App />
        <CyberMessageProvider />
      </ConfigProvider>
    </BrowserRouter>
  </React.StrictMode>
)

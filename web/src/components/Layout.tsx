import React, { ReactNode, useState } from 'react'
import { Layout, Menu, Avatar, Dropdown, Drawer } from 'antd'
import { useNavigate, useLocation } from 'react-router-dom'
import { MenuOutlined, CloudServerOutlined, UserOutlined, LogoutOutlined } from '@ant-design/icons'

const { Header, Sider, Content } = Layout

interface AppLayoutProps {
  onLogout: () => void
  children: ReactNode
}

const AppLayout: React.FC<AppLayoutProps> = ({ onLogout, children }) => {
  const navigate = useNavigate()
  const location = useLocation()
  const [mobileMenuVisible, setMobileMenuVisible] = useState(false)
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768)

  // Handle resize
  React.useEffect(() => {
    const handleResize = () => {
      const mobile = window.innerWidth < 768
      setIsMobile(mobile)
      if (!mobile) setMobileMenuVisible(false)
    }

    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  const menuItems = [
    {
      key: '/dashboard',
      icon: <CloudServerOutlined />,
      label: 'Networks',
      onClick: () => {
        navigate('/dashboard')
        setMobileMenuVisible(false)
      },
    },
  ]

  const userMenuItems = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: 'Logout',
      onClick: onLogout,
    },
  ]

  const sideMenu = (
    <>
      <div style={{
        height: 64,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderBottom: '1px solid rgba(148, 163, 184, 0.1)',
      }}>
        <h2 style={{
          margin: 0,
          fontFamily: 'Orbitron, sans-serif',
          fontSize: isMobile ? '20px' : '24px',
          fontWeight: 600,
          background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          backgroundClip: 'text',
        }}>
          ZeroGo
        </h2>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[location.pathname]}
        items={menuItems}
        style={{
          borderRight: 0,
          background: 'transparent',
          color: '#94a3b8',
        }}
      />
    </>
  )

  return (
    <Layout style={{ minHeight: '100vh', background: '#0a0e27' }}>
      {/* Desktop Sidebar */}
      {!isMobile && (
        <Sider
          width={240}
          style={{
            background: 'rgba(17, 24, 52, 0.8)',
            backdropFilter: 'blur(10px)',
            borderRight: '1px solid rgba(148, 163, 184, 0.1)',
            position: 'fixed',
            height: '100vh',
            left: 0,
            top: 0,
          }}
        >
          {sideMenu}
        </Sider>
      )}

      {/* Mobile Drawer */}
      <Drawer
        title={null}
        placement="left"
        onClose={() => setMobileMenuVisible(false)}
        open={mobileMenuVisible}
        styles={{
          body: { padding: 0, background: 'rgba(17, 24, 52, 0.95)' },
          mask: { background: 'rgba(0, 0, 0, 0.5)' },
        }}
        style={{ background: 'rgba(17, 24, 52, 0.95)' }}
        width={280}
      >
        {sideMenu}
      </Drawer>

      {/* Main Content */}
      <Layout style={{
        background: 'transparent',
        marginLeft: isMobile ? 0 : 240,
        transition: 'margin-left 0.3s',
      }}>
        <Header style={{
          background: 'rgba(26, 34, 64, 0.8)',
          backdropFilter: 'blur(10px)',
          padding: isMobile ? '0 16px' : '0 24px',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          borderBottom: '1px solid rgba(148, 163, 184, 0.1)',
          position: 'sticky',
          top: 0,
          zIndex: 100,
        }}>
          <div>
            {isMobile && (
              <button
                onClick={() => setMobileMenuVisible(true)}
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: '#00f0ff',
                  cursor: 'pointer',
                  fontSize: '20px',
                }}
              >
                <MenuOutlined />
              </button>
            )}
          </div>
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <Avatar
              icon={<UserOutlined />}
              style={{
                cursor: 'pointer',
                background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
                border: '1px solid rgba(0, 240, 255, 0.3)',
              }}
            />
          </Dropdown>
        </Header>
        <Content style={{ padding: 0, background: 'transparent' }}>
          {children}
        </Content>
      </Layout>
    </Layout>
  )
}

export default AppLayout

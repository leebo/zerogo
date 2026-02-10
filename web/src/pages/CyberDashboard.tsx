import React, { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { Plus, Server, Zap, Users } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { Form, Input, Button } from 'antd'
import NetworkCard from '@/components/ui/NetworkCard'
import CyberModal from '@/components/ui/CyberModal'
import { cyberMessage } from '@/components/ui/CyberMessage'
import type { Network, CreateNetworkRequest } from '@/types'
import { networkApi } from '@/api'

const isMobile = () => window.innerWidth < 768

const CyberDashboard: React.FC = () => {
  const navigate = useNavigate()
  const [networks, setNetworks] = useState<Network[]>([])
  const [loading, setLoading] = useState(false)
  const [modalVisible, setModalVisible] = useState(false)
  const [form] = Form.useForm()

  const fetchNetworks = async () => {
    setLoading(true)
    try {
      const data = await networkApi.list()
      setNetworks(data)
    } catch (error) {
      console.error('Fetch networks error:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchNetworks()
    const interval = setInterval(fetchNetworks, 10000)
    return () => clearInterval(interval)
  }, [])

  const handleCreate = () => {
    form.resetFields()

    // Generate random IP range
    const randomOctet2 = Math.floor(Math.random() * 256)
    const randomOctet3 = Math.floor(Math.random() * 256)
    const defaultIPRange = `10.${randomOctet2}.${randomOctet3}.0/24`

    form.setFieldsValue({
      ip_range: defaultIPRange,
    })
    setModalVisible(true)
  }

  const handleSubmit = async (values: CreateNetworkRequest) => {
    try {
      await networkApi.create(values)
      cyberMessage.success('Network created successfully')
      setModalVisible(false)
      fetchNetworks()
    } catch (error) {
      // Error already handled
    }
  }

  const stats = {
    totalNetworks: networks.length,
    totalMembers: networks.reduce((sum, n) => sum + n.member_count, 0),
    onlineMembers: networks.reduce((sum, n) => sum + n.online_count, 0),
  }

  return (
    <div style={{ padding: isMobile() ? '1rem' : '2rem', maxWidth: '1600px', margin: '0 auto' }}>
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.6 }}
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: isMobile() ? 'flex-start' : 'center',
          marginBottom: '2rem',
          gap: isMobile() ? '1rem' : 0,
          flexWrap: isMobile() ? 'wrap' : 'nowrap',
        }}
      >
        <div style={{ flex: isMobile() ? 1 : 'auto' }}>
          <h1 style={{
            fontFamily: 'Orbitron, sans-serif',
            fontSize: isMobile() ? '1.75rem' : '2.5rem',
            fontWeight: 600,
            marginBottom: '0.5rem',
            background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            backgroundClip: 'text',
          }}>
            Network Control
          </h1>
          <p style={{ color: '#94a3b8', fontSize: isMobile() ? '0.875rem' : '1rem' }}>
            Manage your ZeroGo virtual networks
          </p>
        </div>

        <button
          onClick={handleCreate}
          style={{
            background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
            border: 'none',
            borderRadius: '8px',
            color: 'white',
            padding: isMobile() ? '0.625rem 1rem' : '0.75rem 1.5rem',
            fontFamily: 'Orbitron, sans-serif',
            fontWeight: 600,
            letterSpacing: '0.05em',
            cursor: 'pointer',
            transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
            fontSize: isMobile() ? '0.875rem' : '1rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
          }}
        >
          <Plus size={isMobile() ? 16 : 20} />
          <span>{isMobile() ? 'Create' : 'Create Network'}</span>
        </button>
      </motion.div>

      {/* Stats Cards */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: isMobile() ? '1fr' : 'repeat(auto-fit, minmax(250px, 1fr))',
        gap: '1.5rem',
        marginBottom: '2rem',
      }}>
        {[
          {
            label: 'Total Networks',
            value: stats.totalNetworks,
            icon: Server,
            color: '#00f0ff',
          },
          {
            label: 'Total Members',
            value: stats.totalMembers,
            icon: Users,
            color: '#7c3aed',
          },
          {
            label: 'Online Now',
            value: stats.onlineMembers,
            icon: Zap,
            color: '#10b981',
          },
        ].map((stat, index) => (
          <motion.div
            key={stat.label}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, delay: index * 0.1 }}
            className="cyber-card"
            style={{
              padding: isMobile() ? '1rem' : '1.5rem',
              background: 'linear-gradient(135deg, rgba(26, 34, 64, 0.8) 0%, rgba(17, 24, 52, 0.9) 100%)',
              border: '1px solid rgba(148, 163, 184, 0.1)',
              borderRadius: '16px',
              backdropFilter: 'blur(10px)',
              boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
              <div
                style={{
                  width: isMobile() ? '48px' : '56px',
                  height: isMobile() ? '48px' : '56px',
                  borderRadius: '16px',
                  background: 'rgba(0, 240, 255, 0.15)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  border: '1px solid rgba(0, 240, 255, 0.25)',
                }}
              >
                <stat.icon size={isMobile() ? 24 : 28} style={{ color: stat.color }} />
              </div>
              <div>
                <p style={{ color: '#94a3b8', fontSize: '0.875rem', marginBottom: '0.25rem' }}>
                  {stat.label}
                </p>
                <p style={{
                  fontFamily: 'Orbitron, sans-serif',
                  fontSize: isMobile() ? '1.5rem' : '2rem',
                  fontWeight: 600,
                  color: stat.color,
                }}>
                  {stat.value || 0}
                </p>
              </div>
            </div>
          </motion.div>
        ))}
      </div>

      {/* Networks Grid */}
      {loading && networks.length === 0 ? (
        <div className="flex justify-center items-center" style={{ minHeight: '400px' }}>
          <div className="text-secondary">Loading networks...</div>
        </div>
      ) : networks.length === 0 ? (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="cyber-card p-12 text-center"
        >
          <Server size={64} style={{ color: 'var(--color-text-muted)', marginBottom: '1.5rem' }} />
          <h3 className="font-display text-xl mb-2">No Networks Yet</h3>
          <p className="text-secondary mb-6">
            Create your first virtual network to get started
          </p>
          <button onClick={handleCreate} className="cyber-btn">
            <Plus size={20} className="mr-2" />
            Create Network
          </button>
        </motion.div>
      ) : (
        <div style={{
          display: 'grid',
          gridTemplateColumns: isMobile() ? '1fr' : 'repeat(auto-fill, minmax(400px, 1fr))',
          gap: '1.5rem',
        }}>
          {networks.map((network, index) => (
            <NetworkCard
              key={network.id}
              network={network}
              onClick={() => navigate(`/networks/${network.id}`)}
              delay={index * 0.1}
            />
          ))}
        </div>
      )}

      {/* Create Network Modal */}
      <CyberModal
        visible={modalVisible}
        onClose={() => setModalVisible(false)}
        title="Create Network"
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          <Form.Item
            name="name"
            label={<span style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Network Name</span>}
            rules={[{ required: true, message: 'Please input network name!' }]}
          >
            <Input
              placeholder="My Network"
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

          <Form.Item
            name="description"
            label={<span style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Description</span>}
          >
            <Input.TextArea
              placeholder="Network description"
              rows={3}
              style={{
                background: 'rgba(10, 14, 39, 0.5)',
                border: '1px solid rgba(148, 163, 184, 0.2)',
                color: '#f1f5f9',
                borderRadius: '12px',
              }}
            />
          </Form.Item>

          <Form.Item
            name="ip_range"
            label={<span style={{ color: '#94a3b8', fontSize: '0.875rem' }}>IPv4 Range (Auto-generated)</span>}
          >
            <Input
              placeholder="10.x.x.0/24"
              disabled
              size="large"
              style={{
                background: 'rgba(10, 14, 39, 0.3)',
                border: '1px solid rgba(148, 163, 184, 0.1)',
                color: '#64748b',
                borderRadius: '12px',
                height: '48px',
              }}
            />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button
              type="primary"
              htmlType="submit"
              size="large"
              block
              style={{
                height: '48px',
                background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
                border: 'none',
                borderRadius: '12px',
                fontSize: '16px',
                fontWeight: 600,
                fontFamily: 'Orbitron, sans-serif',
                letterSpacing: '0.05em',
              }}
            >
              Create Network
            </Button>
          </Form.Item>
        </Form>
      </CyberModal>
    </div>
  )
}

export default CyberDashboard

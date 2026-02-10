import React, { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Plus, Edit, Trash2, Shield, User as UserIcon } from 'lucide-react'
import { Form, Input, Button } from 'antd'
import CyberModal from '@/components/ui/CyberModal'
import StatusIndicator from '@/components/ui/StatusIndicator'
import { cyberMessage } from '@/components/ui/CyberMessage'
import type { Network, Member, AuthorizeMemberRequest } from '@/types'
import { networkApi, memberApi } from '@/api'
import dayjs from 'dayjs'

const isMobile = () => window.innerWidth < 768

const NetworkDetail: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const [network, setNetwork] = useState<Network | null>(null)
  const [members, setMembers] = useState<Member[]>([])
  const [loading, setLoading] = useState(false)
  const [modalVisible, setModalVisible] = useState(false)
  const [editingMember, setEditingMember] = useState<Member | null>(null)
  const [form] = Form.useForm()

  const networkId = parseInt(id || '0')

  const fetchNetwork = async () => {
    try {
      const data = await networkApi.get(networkId)
      setNetwork(data)
    } catch (error) {
      cyberMessage.error('Network not found')
      navigate('/dashboard')
    }
  }

  const fetchMembers = async () => {
    setLoading(true)
    try {
      const data = await memberApi.list(networkId)
      setMembers(data)
    } catch (error) {
      // Error already handled
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (networkId) {
      fetchNetwork()
      fetchMembers()
      // Refresh data every 10 seconds
      const interval = setInterval(fetchMembers, 10000)
      return () => clearInterval(interval)
    }
  }, [networkId])

  const handleAuthorize = () => {
    setEditingMember(null)
    form.resetFields()
    setModalVisible(true)
  }

  const handleEdit = (member: Member) => {
    setEditingMember(member)
    form.setFieldsValue({
      node_address: member.node_address,
      authorized: member.authorized,
      ip_address: member.ip_address,
      name: member.name,
    })
    setModalVisible(true)
  }

  const handleDelete = async (nodeAddress: string) => {
    try {
      await memberApi.remove(networkId, nodeAddress)
      cyberMessage.success('Member removed successfully')
      fetchMembers()
    } catch (error) {
      // Error already handled
    }
  }

  const handleSubmit = async (values: AuthorizeMemberRequest) => {
    try {
      if (editingMember) {
        await memberApi.update(networkId, editingMember.node_address, values)
        cyberMessage.success('Member updated successfully')
      } else {
        await memberApi.authorize(networkId, values)
        cyberMessage.success('Member authorized successfully')
      }
      setModalVisible(false)
      fetchMembers()
    } catch (error) {
      // Error already handled
    }
  }

  return (
    <div style={{ padding: isMobile() ? '1rem' : '2rem', maxWidth: '1400px', margin: '0 auto' }}>
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.6 }}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '1rem',
          marginBottom: '2rem',
        }}
      >
        <motion.button
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.95 }}
          onClick={() => navigate('/dashboard')}
          style={{
            width: '40px',
            height: '40px',
            borderRadius: '8px',
            background: 'rgba(0, 240, 255, 0.15)',
            border: '1px solid rgba(0, 240, 255, 0.3)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all 0.2s',
          }}
        >
          <ArrowLeft size={20} style={{ color: '#00f0ff' }} />
        </motion.button>

        <div style={{ flex: 1 }}>
          <h1 style={{
            fontFamily: 'Orbitron, sans-serif',
            fontSize: isMobile() ? '1.5rem' : '2rem',
            fontWeight: 600,
            marginBottom: '0.25rem',
            background: 'linear-gradient(135deg, #00f0ff 0%, #7c3aed 100%)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            backgroundClip: 'text',
          }}>
            {network?.name || 'Loading...'}
          </h1>
          <p style={{ color: '#94a3b8', fontSize: '0.875rem' }}>
            Network ID: {networkId}
          </p>
        </div>

        <button
          onClick={handleAuthorize}
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
            transition: 'all 0.3s',
            fontSize: isMobile() ? '0.875rem' : '1rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
          }}
        >
          <Plus size={isMobile() ? 16 : 20} />
          <span>{isMobile() ? 'Add' : 'Authorize Member'}</span>
        </button>
      </motion.div>

      {/* Network Info Card */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.6, delay: 0.1 }}
        className="cyber-card"
        style={{
          padding: isMobile() ? '1.5rem' : '2rem',
          marginBottom: '2rem',
        }}
      >
        <div style={{
          display: 'grid',
          gridTemplateColumns: isMobile() ? '1fr' : 'repeat(auto-fit, minmax(200px, 1fr))',
          gap: '2rem',
        }}>
          <div>
            <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.5rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              Network ID
            </p>
            <p style={{ color: '#f1f5f9', fontSize: '1.125rem', fontFamily: 'Orbitron, sans-serif', fontWeight: 500 }}>
              {network?.id}
            </p>
          </div>

          <div>
            <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.5rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              IP Range
            </p>
            <p className="mono" style={{ color: '#00f0ff', fontSize: '1rem' }}>
              {network?.ip_range}
            </p>
          </div>

          <div>
            <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.5rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              MTU
            </p>
            <p style={{ color: '#f1f5f9', fontSize: '1.125rem', fontFamily: 'Orbitron, sans-serif', fontWeight: 500 }}>
              {network?.mtu}
            </p>
          </div>

          <div>
            <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.5rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              Multicast
            </p>
            <p style={{
              color: network?.multicast ? '#10b981' : '#64748b',
              fontSize: '1rem',
            }}>
              {network?.multicast ? 'Enabled' : 'Disabled'}
            </p>
          </div>

          {network?.description && (
            <div style={{ gridColumn: isMobile() ? '1 / -1' : '1 / -1' }}>
              <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.5rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Description
              </p>
              <p style={{ color: '#94a3b8', fontSize: '0.9375rem' }}>
                {network.description}
              </p>
            </div>
          )}
        </div>
      </motion.div>

      {/* Members Card */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.6, delay: 0.2 }}
        className="cyber-card"
        style={{
          padding: isMobile() ? '1.5rem' : '2rem',
        }}
      >
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: '1.5rem',
        }}>
          <h2 style={{
            fontFamily: 'Orbitron, sans-serif',
            fontSize: isMobile() ? '1.25rem' : '1.5rem',
            fontWeight: 600,
            margin: 0,
          }}>
            Members ({members.length})
          </h2>

          {loading && (
            <div style={{ color: '#64748b', fontSize: '0.875rem' }}>
              Loading...
            </div>
          )}
        </div>

        {/* Members List */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {members.length === 0 ? (
            <div style={{
              padding: '3rem',
              textAlign: 'center',
              color: '#64748b',
            }}>
              No members yet. Authorize your first member to get started.
            </div>
          ) : (
            members.map((member, index) => (
              <motion.div
                key={member.node_address}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.4, delay: index * 0.05 }}
                style={{
                  background: 'rgba(10, 14, 39, 0.5)',
                  border: '1px solid rgba(148, 163, 184, 0.1)',
                  borderRadius: '12px',
                  padding: '1.5rem',
                  display: 'grid',
                  gridTemplateColumns: isMobile() ? '1fr' : '2fr 1fr 1fr 1fr auto',
                  gap: '1.5rem',
                  alignItems: 'center',
                  transition: 'all 0.2s',
                }}
              >
                {/* Member Info */}
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.5rem' }}>
                    <Shield size={18} style={{ color: '#64748b' }} />
                    <span className="mono" style={{ color: '#64748b', fontSize: '0.75rem' }}>
                      {member.node_address.substring(0, 12)}...
                    </span>
                  </div>
                  <p style={{ color: '#f1f5f9', fontSize: '1rem', fontWeight: 500 }}>
                    {member.name || 'Unnamed'}
                  </p>
                </div>

                {/* IP Address */}
                <div>
                  <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.25rem' }}>
                    IP Address
                  </p>
                  <p className="mono" style={{ color: '#00f0ff', fontSize: '0.9375rem' }}>
                    {member.ip_address || 'Not assigned'}
                  </p>
                </div>

                {/* Status */}
                <div>
                  <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.25rem' }}>
                    Status
                  </p>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <span style={{
                      padding: '0.25rem 0.75rem',
                      borderRadius: '6px',
                      fontSize: '0.75rem',
                      background: member.authorized ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)',
                      color: member.authorized ? '#10b981' : '#ef4444',
                      fontWeight: 500,
                    }}>
                      {member.authorized ? 'Authorized' : 'Pending'}
                    </span>
                    <StatusIndicator online={member.online} size="sm" showText />
                  </div>
                </div>

                {/* Platform */}
                <div>
                  <p style={{ color: '#64748b', fontSize: '0.75rem', marginBottom: '0.25rem' }}>
                    Platform
                  </p>
                  <p style={{ color: '#94a3b8', fontSize: '0.875rem' }}>
                    {member.platform || 'Unknown'}
                  </p>
                </div>

                {/* Actions */}
                <div style={{
                  display: 'flex',
                  gap: '0.5rem',
                  justifyContent: 'flex-end',
                }}>
                  <button
                    onClick={() => handleEdit(member)}
                    style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '8px',
                      background: 'rgba(0, 240, 255, 0.15)',
                      border: '1px solid rgba(0, 240, 255, 0.3)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      cursor: 'pointer',
                      transition: 'all 0.2s',
                    }}
                  >
                    <Edit size={16} style={{ color: '#00f0ff' }} />
                  </button>

                  <button
                    onClick={() => handleDelete(member.node_address)}
                    style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '8px',
                      background: 'rgba(239, 68, 68, 0.15)',
                      border: '1px solid rgba(239, 68, 68, 0.3)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      cursor: 'pointer',
                      transition: 'all 0.2s',
                    }}
                  >
                    <Trash2 size={16} style={{ color: '#ef4444' }} />
                  </button>
                </div>
              </motion.div>
            ))
          )}
        </div>
      </motion.div>

      {/* Authorize Member Modal */}
      <CyberModal
        visible={modalVisible}
        onClose={() => setModalVisible(false)}
        title={editingMember ? 'Edit Member' : 'Authorize Member'}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          <Form.Item
            name="node_address"
            label={<span style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Node Address</span>}
            rules={[{ required: true, message: 'Please input node address!' }]}
          >
            <Input
              placeholder="Node identity address"
              disabled={!!editingMember}
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
            name="name"
            label={<span style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Member Name</span>}
            rules={[{ required: true, message: 'Please input member name!' }]}
          >
            <Input
              placeholder="Member name"
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
            name="ip_address"
            label={<span style={{ color: '#94a3b8', fontSize: '0.875rem' }}>IP Address</span>}
          >
            <Input
              placeholder="Auto-allocate if empty"
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
            name="authorized"
            label={<span style={{ color: '#94a3b8', fontSize: '0.875rem' }}>Authorized</span>}
            valuePropName="checked"
            initialValue={true}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <input
                type="checkbox"
                style={{ width: '20px', height: '20px', cursor: 'pointer' }}
              />
              <span style={{ color: '#94a3b8', fontSize: '0.9375rem' }}>
                Authorize this member to join the network
              </span>
            </div>
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
              {editingMember ? 'Update Member' : 'Authorize Member'}
            </Button>
          </Form.Item>
        </Form>
      </CyberModal>
    </div>
  )
}

export default NetworkDetail

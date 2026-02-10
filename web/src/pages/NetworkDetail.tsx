import React, { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Card, Table, Button, Space, Tag, Modal, Form, Input, Switch, message, Popconfirm } from 'antd'
import { ArrowLeftOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import type { Network, Member, AuthorizeMemberRequest } from '@/types'
import { networkApi, memberApi } from '@/api'
import dayjs from 'dayjs'

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
      message.error('Network not found')
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
      message.success('Member removed successfully')
      fetchMembers()
    } catch (error) {
      // Error already handled
    }
  }

  const handleSubmit = async (values: AuthorizeMemberRequest) => {
    try {
      if (editingMember) {
        await memberApi.update(networkId, editingMember.node_address, values)
        message.success('Member updated successfully')
      } else {
        await memberApi.authorize(networkId, values)
        message.success('Member authorized successfully')
      }
      setModalVisible(false)
      fetchMembers()
    } catch (error) {
      // Error already handled
    }
  }

  const columns = [
    {
      title: 'Node Address',
      dataIndex: 'node_address',
      key: 'node_address',
      ellipsis: true,
    },
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'IP Address',
      dataIndex: 'ip_address',
      key: 'ip_address',
    },
    {
      title: 'Status',
      key: 'status',
      render: (_: any, record: Member) => (
        <Space>
          <Tag color={record.authorized ? 'green' : 'red'}>
            {record.authorized ? 'Authorized' : 'Unauthorized'}
          </Tag>
          <Tag color={record.online ? 'blue' : 'default'}>
            {record.online ? 'Online' : 'Offline'}
          </Tag>
        </Space>
      ),
    },
    {
      title: 'Platform',
      dataIndex: 'platform',
      key: 'platform',
    },
    {
      title: 'Last Seen',
      dataIndex: 'last_seen',
      key: 'last_seen',
      render: (date: string) => dayjs(date).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_: any, record: Member) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
          <Popconfirm
            title="Are you sure to remove this member?"
            onConfirm={() => handleDelete(record.node_address)}
            okText="Yes"
            cancelText="No"
          >
            <Button type="link" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Card
        title={
          <Space>
            <Button
              type="text"
              icon={<ArrowLeftOutlined />}
              onClick={() => navigate('/dashboard')}
            />
            {network?.name}
          </Space>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAuthorize}>
            Authorize Member
          </Button>
        }
        style={{ marginBottom: 16 }}
      >
        <p><strong>ID:</strong> {network?.id}</p>
        <p><strong>Description:</strong> {network?.description}</p>
        <p><strong>IP Range:</strong> {network?.ip_range}</p>
        <p><strong>MTU:</strong> {network?.mtu}</p>
        <p><strong>Multicast:</strong> {network?.multicast ? 'Enabled' : 'Disabled'}</p>
      </Card>

      <Card title="Members">
        <Table
          columns={columns}
          dataSource={members}
          rowKey="node_address"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={editingMember ? 'Edit Member' : 'Authorize Member'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          <Form.Item
            name="node_address"
            label="Node Address"
            rules={[{ required: true, message: 'Please input node address!' }]}
          >
            <Input placeholder="Node identity address" disabled={!!editingMember} />
          </Form.Item>

          <Form.Item
            name="name"
            label="Name"
            rules={[{ required: true, message: 'Please input member name!' }]}
          >
            <Input placeholder="Member name" />
          </Form.Item>

          <Form.Item
            name="ip_address"
            label="IP Address"
          >
            <Input placeholder="Auto-allocate if empty" />
          </Form.Item>

          <Form.Item
            name="authorized"
            label="Authorized"
            valuePropName="checked"
            initialValue={true}
          >
            <Switch />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" block>
              {editingMember ? 'Update' : 'Authorize'}
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default NetworkDetail

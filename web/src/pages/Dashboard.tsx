import React, { useEffect, useState } from 'react'
import { Table, Button, Card, Space, Tag, Modal, Form, Input, InputNumber, message, Popconfirm } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { Network, CreateNetworkRequest } from '@/types'
import { networkApi } from '@/api'
import dayjs from 'dayjs'

const Dashboard: React.FC = () => {
  const [networks, setNetworks] = useState<Network[]>([])
  const [loading, setLoading] = useState(false)
  const [modalVisible, setModalVisible] = useState(false)
  const [editingNetwork, setEditingNetwork] = useState<Network | null>(null)
  const [form] = Form.useForm()
  const navigate = useNavigate()

  const fetchNetworks = async () => {
    setLoading(true)
    try {
      const data = await networkApi.list()
      setNetworks(data)
    } catch (error) {
      // Error already handled
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchNetworks()
    // Refresh data every 10 seconds
    const interval = setInterval(fetchNetworks, 10000)
    return () => clearInterval(interval)
  }, [])

  const handleCreate = () => {
    setEditingNetwork(null)
    form.resetFields()
    setModalVisible(true)
  }

  const handleEdit = (network: Network) => {
    setEditingNetwork(network)
    form.setFieldsValue({
      name: network.name,
      description: network.description,
      ip_range: network.ip_range,
      ip6_range: network.ip6_range,
      mtu: network.mtu,
      multicast: network.multicast,
    })
    setModalVisible(true)
  }

  const handleDelete = async (id: number) => {
    try {
      await networkApi.delete(id)
      message.success('Network deleted successfully')
      fetchNetworks()
    } catch (error) {
      // Error already handled
    }
  }

  const handleSubmit = async (values: CreateNetworkRequest) => {
    try {
      if (editingNetwork) {
        await networkApi.update(editingNetwork.id, values)
        message.success('Network updated successfully')
      } else {
        await networkApi.create(values)
        message.success('Network created successfully')
      }
      setModalVisible(false)
      fetchNetworks()
    } catch (error) {
      // Error already handled
    }
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: Network) => (
        <a onClick={() => navigate(`/networks/${record.id}`)}>{text}</a>
      ),
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'IP Range',
      dataIndex: 'ip_range',
      key: 'ip_range',
    },
    {
      title: 'MTU',
      dataIndex: 'mtu',
      key: 'mtu',
      width: 80,
    },
    {
      title: 'Members',
      key: 'members',
      width: 120,
      render: (_: any, record: Network) => (
        <Space>
          <Tag color="blue">{record.member_count} Total</Tag>
          <Tag color="green">{record.online_count} Online</Tag>
        </Space>
      ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (date: string) => dayjs(date).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 180,
      render: (_: any, record: Network) => (
        <Space>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => navigate(`/networks/${record.id}`)}
          />
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
          <Popconfirm
            title="Are you sure to delete this network?"
            onConfirm={() => handleDelete(record.id)}
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
        title="Networks"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            Create Network
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={networks}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={editingNetwork ? 'Edit Network' : 'Create Network'}
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
            name="name"
            label="Network Name"
            rules={[{ required: true, message: 'Please input network name!' }]}
          >
            <Input placeholder="My Network" />
          </Form.Item>

          <Form.Item
            name="description"
            label="Description"
          >
            <Input.TextArea placeholder="Network description" rows={3} />
          </Form.Item>

          <Form.Item
            name="ip_range"
            label="IPv4 Range"
            rules={[{ required: true, message: 'Please input IPv4 range!' }]}
          >
            <Input placeholder="10.147.0.0/24" />
          </Form.Item>

          <Form.Item
            name="ip6_range"
            label="IPv6 Range"
          >
            <Input placeholder="fdaa:bbcc:dd::/64" />
          </Form.Item>

          <Form.Item
            name="mtu"
            label="MTU"
            initialValue={2800}
          >
            <InputNumber min={1280} max={9000} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            name="multicast"
            label="Multicast"
            valuePropName="checked"
            initialValue={true}
          >
            <InputNumber />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" block>
              {editingNetwork ? 'Update' : 'Create'}
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Dashboard

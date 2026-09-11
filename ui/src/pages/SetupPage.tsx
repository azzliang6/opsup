import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import Form from 'antd/es/form'
import Input from 'antd/es/input'
import Button from 'antd/es/button'
import Card from 'antd/es/card'
import Typography from 'antd/es/typography'
import message from 'antd/es/message'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useAuth } from '../hooks/useAuth'
import { getAuthStatus } from '../api/auth'
import { useTheme } from '../contexts/ThemeContext'

const { Title, Text } = Typography

export default function SetupPage() {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const { doSetup } = useAuth()
  const { colors } = useTheme()

  useEffect(() => {
    getAuthStatus().then((res) => {
      if (res.initialized) {
        navigate('/login')
      }
    })
  }, [navigate])

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      await doSetup(values.username, values.password)
      message.success('管理员创建成功')
      navigate('/')
    } catch (e: any) {
      message.error(e.response?.data?.error || '创建失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      style={{
        height: '100dvh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: colors.gradientPage,
      }}
    >
      <Card
        style={{
          width: 420,
          background: colors.bgCardOverlay,
          border: `1px solid ${colors.borderSubtle}`,
          borderRadius: 12,
          backdropFilter: 'blur(10px)',
        }}
      >
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <Title level={3} style={{ color: colors.textPrimary, margin: 0 }}>
            初始化 OpsUp
          </Title>
          <Text style={{ color: colors.textTertiary }}>创建管理员账户</Text>
        </div>

        <Form onFinish={onFinish} size="large">
          <Form.Item name="username" rules={[{ required: true, min: 3, message: '用户名至少3个字符' }]}>
            <Input prefix={<UserOutlined />} placeholder="管理员用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[
            { required: true, min: 12, message: '密码至少12个字符' },
            { validator: (_, value: string) => !value || new TextEncoder().encode(value).length <= 72
              ? Promise.resolve() : Promise.reject(new Error('密码最多72个 UTF-8 字节')) },
          ]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item
            name="confirm"
            dependencies={['password']}
            rules={[
              { required: true, message: '请确认密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="确认密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              创建管理员
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}

import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Form, Input, Button, Card, Typography, message } from 'antd'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useAuth } from '../hooks/useAuth'
import { getAuthStatus } from '../api/auth'
import { useTheme } from '../contexts/ThemeContext'

const { Title, Text } = Typography

export default function LoginPage() {
  const [loading, setLoading] = useState(false)
  const [initialized, setInitialized] = useState<boolean | null>(null)
  const navigate = useNavigate()
  const { doLogin } = useAuth()
  const { colors } = useTheme()

  useEffect(() => {
    getAuthStatus().then((res) => {
      setInitialized(res.initialized)
      if (!res.initialized) {
        navigate('/setup', { replace: true })
      }
    }).catch(() => {
      // API error, assume initialized and show login
      setInitialized(true)
    })
  }, [navigate])

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      await doLogin(values.username, values.password)
      message.success('登录成功')
      navigate('/')
    } catch (e: any) {
      message.error(e.response?.data?.error || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  if (initialized === false) return null

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
          width: 400,
          background: colors.bgCardOverlay,
          border: `1px solid ${colors.borderSubtle}`,
          borderRadius: 12,
          backdropFilter: 'blur(10px)',
        }}
      >
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <div
            style={{
              width: 56,
              height: 56,
              borderRadius: 14,
              background: colors.gradientLogo,
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 28,
              color: '#fff',
              fontFamily: 'monospace',
              fontWeight: 'bold',
              marginBottom: 16,
            }}
          >
            &gt;_
          </div>
          <Title level={3} style={{ color: colors.textPrimary, margin: 0 }}>
            OpsUp
          </Title>
          <Text style={{ color: colors.textTertiary }}>Web SSH Terminal</Text>
        </div>

        <Form onFinish={onFinish} size="large">
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}

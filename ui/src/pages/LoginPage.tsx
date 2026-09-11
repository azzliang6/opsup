import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import Form from 'antd/es/form'
import Input from 'antd/es/input'
import Button from 'antd/es/button'
import message from 'antd/es/message'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useAuth } from '../hooks/useAuth'
import { getAuthStatus } from '../api/auth'
import AuthFrame from '../components/AuthFrame'

export default function LoginPage() {
  const [loading, setLoading] = useState(false)
  const [initialized, setInitialized] = useState<boolean | null>(null)
  const navigate = useNavigate()
  const { doLogin } = useAuth()

  useEffect(() => {
    getAuthStatus().then((res) => {
      setInitialized(res.initialized)
      if (!res.initialized) navigate('/setup', { replace: true })
    }).catch(() => setInitialized(true))
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
    <AuthFrame title="OpsUp" description="登录工作台，连接你的服务器">
      <Form onFinish={onFinish} size="large" layout="vertical" requiredMark={false}>
        <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
          <Input prefix={<UserOutlined />} placeholder="用户名" autoComplete="username" />
        </Form.Item>
        <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
          <Input.Password prefix={<LockOutlined />} placeholder="密码" autoComplete="current-password" />
        </Form.Item>
        <Form.Item className="auth-submit">
          <Button type="primary" htmlType="submit" loading={loading} block>登录</Button>
        </Form.Item>
      </Form>
    </AuthFrame>
  )
}

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

export default function SetupPage() {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const { doSetup } = useAuth()

  useEffect(() => {
    getAuthStatus().then((res) => {
      if (res.initialized) navigate('/login')
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
    <AuthFrame title="初始化 OpsUp" description="创建管理员账户，开始管理 SSH 连接">
      <Form onFinish={onFinish} size="large" layout="vertical" requiredMark={false}>
        <Form.Item name="username" label="管理员用户名" rules={[{ required: true, min: 3, message: '用户名至少3个字符' }]}>
          <Input prefix={<UserOutlined />} placeholder="管理员用户名" autoComplete="username" />
        </Form.Item>
        <Form.Item name="password" label="密码" rules={[
          { required: true, min: 12, message: '密码至少12个字符' },
          { validator: (_, value: string) => !value || new TextEncoder().encode(value).length <= 72
            ? Promise.resolve() : Promise.reject(new Error('密码最多72个 UTF-8 字节')) },
        ]}>
          <Input.Password prefix={<LockOutlined />} placeholder="密码" autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="confirm" label="确认密码" dependencies={['password']} rules={[
          { required: true, message: '请确认密码' },
          ({ getFieldValue }) => ({ validator(_, value) {
            return !value || getFieldValue('password') === value
              ? Promise.resolve() : Promise.reject(new Error('两次密码不一致'))
          } }),
        ]}>
          <Input.Password prefix={<LockOutlined />} placeholder="确认密码" autoComplete="new-password" />
        </Form.Item>
        <Form.Item className="auth-submit">
          <Button type="primary" htmlType="submit" loading={loading} block>创建管理员</Button>
        </Form.Item>
      </Form>
    </AuthFrame>
  )
}

import { useEffect, useState, useRef } from 'react'
import Modal from 'antd/es/modal'
import Form from 'antd/es/form'
import Input from 'antd/es/input'
import InputNumber from 'antd/es/input-number'
import Select from 'antd/es/select'
import Button from 'antd/es/button'
import Radio from 'antd/es/radio'
import AutoComplete from 'antd/es/auto-complete'
import message from 'antd/es/message'
import { CopyOutlined, EditOutlined, SwapOutlined, UploadOutlined, KeyOutlined, LockOutlined } from '@ant-design/icons'
import * as serverApi from '../api/servers'
import { useTheme } from '../contexts/ThemeContext'
import { HOST_KEY_PATTERN } from '../utils/hostKey'
import type { Server, ServerForm } from '../types'

interface Props {
  open: boolean
  server: Server | null
  onOk: () => void
  onCancel: () => void
}

type KeySource = 'new' | 'reuse'
type AuthType = 'key' | 'password'

export default function ServerFormModal({ open, server, onOk, onCancel }: Props) {
  const [form] = Form.useForm<ServerForm>()
  const [loading, setLoading] = useState(false)
  const { colors } = useTheme()
  const [authType, setAuthType] = useState<AuthType>('key')
  const [keySource, setKeySource] = useState<KeySource>('new')
  const [existingServers, setExistingServers] = useState<Server[]>([])
  const [reuseServerId, setReuseServerId] = useState<number | null>(null)
  const [privateKeyValue, setPrivateKeyValue] = useState('')
  const [jumpServerId, setJumpServerId] = useState<number | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const isEdit = !!server

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => {
      const text = (ev.target?.result as string).trim()
      setPrivateKeyValue(text)
      form.setFieldValue('private_key', text)
      setKeySource('new')
      message.success('密钥文件已导入')
    }
    reader.onerror = () => {
      message.error('读取文件失败')
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  useEffect(() => {
    let active = true
    if (open) {
      serverApi.listServers().then((list) => { if (active) setExistingServers(list || []) }).catch(() => { if (active) message.error('加载服务器列表失败') })

      if (server) {
        form.setFieldsValue({
          name: server.name,
          host: server.host,
          host_key: server.host_key || '',
          port: server.port,
          username: server.username,
          auth_type: server.auth_type || 'key',
          private_key: '',
          password: '',
          copy_key_from: 0,
          jump_server_id: server.jump_server_id || null,
          description: server.description,
          group: server.group || '',
        })
        setAuthType((server.auth_type as AuthType) || 'key')
        setKeySource('new')
        setReuseServerId(null)
        setPrivateKeyValue('')
        setJumpServerId(server.jump_server_id || null)
      } else {
        form.resetFields()
        form.setFieldsValue({ port: 22, auth_type: 'key', host_key: '' })
        setAuthType('key')
        setKeySource(existingServers.length > 0 ? 'reuse' : 'new')
        setReuseServerId(null)
        setPrivateKeyValue('')
        setJumpServerId(null)
      }
    }
    return () => { active = false }
  }, [open, server, form])

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      values.auth_type = authType
      values.host_key = (values.host_key || '').trim()

      if (authType === 'password') {
        values.private_key = ''
        values.copy_key_from = 0
        if (!isEdit && !values.password) {
          message.error('请输入密码')
          return
        }
      } else {
        values.password = ''
        const pk = form.getFieldValue('private_key') as string | undefined
        if (keySource === 'reuse' && reuseServerId) {
          values.private_key = ''
          values.copy_key_from = reuseServerId
        } else {
          values.copy_key_from = 0
          values.private_key = pk || ''
          if (!isEdit && !values.private_key) {
            message.error('请输入SSH私钥或选择复用已有密钥')
            return
          }
        }
      }

      values.jump_server_id = jumpServerId

      setLoading(true)
      if (isEdit) {
        await serverApi.updateServer(server!.id, values)
        message.success('更新成功')
      } else {
        await serverApi.createServer(values)
        message.success('添加成功')
      }
      onOk()
    } catch (e: any) {
      if (e.response) {
        message.error(e.response?.data?.error || '操作失败')
      }
    } finally {
      setLoading(false)
    }
  }

  const reuseOptions = existingServers.filter((s) => !server || s.id !== server.id)

  return (
    <Modal
      title={isEdit ? '编辑服务器' : '添加服务器'}
      open={open}
      onCancel={onCancel}
      onOk={handleSubmit}
      confirmLoading={loading}
      okText={isEdit ? '更新' : '添加'}
      width={520}
      styles={{ body: { maxHeight: '60vh', overflowY: 'auto', paddingRight: 4 } }}
    >
      <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
        <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
          <Input placeholder="如: 生产环境 Web 服务器" />
        </Form.Item>

        <div style={{ display: 'flex', gap: 12 }}>
          <Form.Item
            name="host"
            label="主机"
            rules={[{ required: true, message: '请输入主机地址' }]}
            style={{ flex: 1 }}
          >
            <Input placeholder="192.168.1.100 或 example.com" />
          </Form.Item>
          <Form.Item name="port" label="端口" style={{ width: 100 }}>
            <InputNumber min={1} max={65535} />
          </Form.Item>
        </div>

        <Form.Item
          name="username"
          label="用户名"
          rules={[{ required: true, message: '请输入SSH用户名' }]}
        >
          <Input placeholder="root" />
        </Form.Item>

        <Form.Item
          name="host_key"
          label="SSH 主机密钥指纹"
          normalize={(value: string) => value.trim()}
          rules={[{ pattern: HOST_KEY_PATTERN, message: '请输入 SHA256: 后跟 43 位 Base64 字符的指纹' }]}
          extra={<span>可留空保存，但未验证指纹时无法连接。请通过独立可信控制台核对主机公钥，例如 <code>ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub</code>。实际协商也可能使用 ECDSA 或 RSA，需核对对应公钥文件的 SHA256 指纹。连接错误中的观测指纹仅供比对，请勿直接信任或自动接受。</span>}
        >
          <Input placeholder="SHA256:…" autoComplete="off" />
        </Form.Item>

        {/* Auth type selector */}
        <Form.Item label="认证方式">
          <Radio.Group
            value={authType}
            onChange={(e) => {
              setAuthType(e.target.value)
            }}
          >
            <Radio value="key">
              <KeyOutlined style={{ marginRight: 4 }} />
              密钥认证
            </Radio>
            <Radio value="password">
              <LockOutlined style={{ marginRight: 4 }} />
              密码认证
            </Radio>
          </Radio.Group>

          {authType === 'password' ? (
            <Form.Item name="password" noStyle>
              <Input.Password
                placeholder="SSH 登录密码"
                style={{ marginTop: 8 }}
              />
            </Form.Item>
          ) : (
            <>
              <Radio.Group
                value={keySource}
                onChange={(e) => setKeySource(e.target.value)}
                style={{ marginTop: 8, marginBottom: 8 }}
              >
                <Radio value="reuse">
                  <CopyOutlined style={{ marginRight: 4 }} />
                  复用已有密钥
                </Radio>
                <Radio value="new">
                  <EditOutlined style={{ marginRight: 4 }} />
                  粘贴新密钥
                </Radio>
              </Radio.Group>

              {keySource === 'reuse' ? (
                reuseOptions.length > 0 ? (
                  <Select
                    placeholder="选择要复用密钥的服务器"
                    value={reuseServerId}
                    onChange={setReuseServerId}
                    style={{ width: '100%' }}
                    options={reuseOptions.map((s) => ({
                      value: s.id,
                      label: `${s.name} (${s.username}@${s.host})`,
                    }))}
                  />
                ) : (
                  <div style={{ color: colors.textTertiary, fontSize: 13 }}>
                    暂无已有服务器，请先粘贴密钥
                  </div>
                )
              ) : (
                <>
                  <Input.TextArea
                    rows={5}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
                    style={{ fontFamily: 'monospace', fontSize: 12 }}
                    value={privateKeyValue}
                    onChange={(e) => {
                      setPrivateKeyValue(e.target.value)
                      form.setFieldValue('private_key', e.target.value)
                    }}
                  />
                  <Button
                    size="small"
                    icon={<UploadOutlined />}
                    style={{ marginTop: 6 }}
                    onClick={() => fileInputRef.current?.click()}
                  >
                    选择密钥文件
                  </Button>
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept=".pem,.key,.id_rsa,.pub,.txt"
                    style={{ display: 'none' }}
                    onChange={handleFileSelect}
                  />
                </>
              )}
              {isEdit && keySource === 'new' && (
                <div style={{ fontSize: 12, color: colors.textTertiary, marginTop: 4 }}>
                  留空则保留原有密钥不变
                </div>
              )}
            </>
          )}
        </Form.Item>

        {/* Jump server */}
        <Form.Item label={<span><SwapOutlined style={{ marginRight: 4 }} />跳板机</span>} extra="通过跳板机连接VPN后的服务器，不选则为直连">
          <Select
            allowClear
            placeholder="无（直连）"
            value={jumpServerId}
            onChange={(val: number | null) => setJumpServerId(val)}
            style={{ width: '100%' }}
            options={reuseOptions.map((s) => ({
              value: s.id,
              label: `${s.name} (${s.username}@${s.host})`,
            }))}
          />
        </Form.Item>

        <Form.Item name="description" label="描述">
          <Input.TextArea rows={2} placeholder="可选描述信息" />
        </Form.Item>

        <Form.Item name="group" label="分组">
          <AutoComplete
            placeholder="输入分组名，如：生产环境、测试环境"
            options={[...new Set(existingServers.map((s) => s.group).filter(Boolean))].map((g) => ({
              value: g,
            }))}
            style={{ width: '100%' }}
            filterOption={(input, option) =>
              option?.value?.toLowerCase().includes(input.toLowerCase()) ?? false
            }
          />
        </Form.Item>
      </Form>
    </Modal>
  )
}

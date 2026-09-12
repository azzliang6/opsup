import type { UserInteraction } from '@devolutions/iron-remote-desktop'
import type { Server } from '../types'
import './rdp.css'

const el = <T extends HTMLElement>(id: string) => document.getElementById(id) as T
const login = el<HTMLFormElement>('login')
const password = el<HTMLInputElement>('password')
const connectButton = el<HTMLButtonElement>('connect')
const desktop = el<HTMLElement>('desktop')
let server: Server | undefined
let token = ''
let active = true
let api: UserInteraction | undefined
let connected = false
let connecting = false
let backend: typeof import('@devolutions/iron-remote-desktop-rdp')

function status(text: string, value: 'connecting' | 'connected' | 'error') {
  el('status').textContent = text
  parent.postMessage({ type: 'opsup-rdp-status', status: value }, location.origin)
}
function size() {
  return { width: Math.max(320, Math.min(3840, Math.floor(desktop.clientWidth / 2) * 2)), height: Math.max(200, Math.min(2160, Math.floor(desktop.clientHeight / 2) * 2)) }
}

window.addEventListener('message', event => {
  if (event.source !== parent || event.origin !== location.origin || event.data?.type !== 'opsup-rdp-init') return
  const data = event.data
  if (!server) {
    server = data.server
    el('name').textContent = server!.name + ' · RDP'
    el<HTMLInputElement>('username').value = server!.username
    el<HTMLInputElement>('domain').value = server!.rdp_domain || ''
  }
  token = data.token
  active = data.active
  if (!active) { (document.activeElement as HTMLElement)?.blur(); window.dispatchEvent(new Event('blur')) }
  if (connected && active) { const s = size(); api?.resize(s.width, s.height) }
})
parent.postMessage({ type: 'opsup-rdp-ready' }, location.origin)

async function load() {
  if (!window.isSecureContext) throw new Error('HTTPS required')
  backend = await import('@devolutions/iron-remote-desktop-rdp')
  await backend.init('OFF')
  await import('@devolutions/iron-remote-desktop')
  const component = document.createElement('iron-remote-desktop') as HTMLElement & { module: typeof backend.Backend }
  component.module = backend.Backend
  component.setAttribute('scale', 'fit')
  component.setAttribute('verbose', 'false')
  component.addEventListener('ready', event => {
    api = (event as CustomEvent<{ irgUserInteraction: UserInteraction }>).detail.irgUserInteraction
    api.setEnableAutoClipboard(false)
    api.setEnableClipboard(true)
    api.onWarningCallback(() => { el('status').textContent = '剪贴板操作不可用，请检查浏览器权限' })
    connectButton.disabled = false
    status('请输入密码连接', 'connecting')
  })
  desktop.append(component)
}
void load().catch(() => status('客户端加载失败；请使用 HTTPS 并刷新重试', 'error'))

login.addEventListener('submit', async event => {
  event.preventDefault()
  if (!api || !server || connecting || connected) return
  connecting = true
  connectButton.disabled = true
  el('cancel').hidden = false
  status('正在连接…', 'connecting')
  const url = new URL(`/api/rdp/${server.id}`, location.origin)
  url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = server.host.includes(':') ? `[${server.host}]` : server.host
  // Password and proxy token stay in memory. Neither is placed in a URL,
  // saved in browser storage, nor returned from OpsUp's server API.
  let config = api.configBuilder().withUsername(el<HTMLInputElement>('username').value)
    .withPassword(password.value).withServerDomain(el<HTMLInputElement>('domain').value)
    .withDestination(`${host}:${server.port}`).withProxyAddress(url.href).withAuthToken(token)
    .withDesktopSize(size()).withExtension(backend.displayControl(true)).withExtension(backend.enableCredssp(true)).build()
  password.value = ''
  try {
    const pending = api.connect(config)
    config = undefined as unknown as typeof config
    const session = await pending
    connected = true
    connecting = false
    el('cancel').hidden = true
    login.hidden = true
    el('tools').hidden = false
    api.setVisibility(true)
    status('已连接', 'connected')
    const run = session.run()
    const s = size(); api.resize(s.width, s.height)
    await run
    status('连接已断开，请重新输入密码', 'error')
  } catch {
    status('连接失败：请检查网络、NLA、账号密码及证书指纹', 'error')
  } finally {
    // Drop references retained by this async invocation.
    config = undefined as unknown as typeof config
    api.shutdown()
    api.setVisibility(false)
    connected = false
    connecting = false
    login.hidden = false
    el('tools').hidden = true
    connectButton.disabled = false
    el('cancel').hidden = true
  }
})
el('disconnect').onclick = () => api?.shutdown()
el('cancel').onclick = () => location.reload()
el('cad').onclick = () => api?.ctrlAltDel()
el('fullscreen').onclick = () => { void document.documentElement.requestFullscreen().catch(() => { el('status').textContent = '浏览器未允许全屏' }) }
el('paste').onclick = () => { if (active) void api?.sendClipboardData().catch(() => { el('status').textContent = '无法读取本地剪贴板，请检查权限' }) }
el('copy').onclick = () => { if (active) void api?.saveRemoteClipboardData().catch(() => { el('status').textContent = '无法写入本地剪贴板，请检查权限' }) }
let timer: ReturnType<typeof setTimeout>
new ResizeObserver(() => {
  clearTimeout(timer)
  timer = setTimeout(() => { if (connected && active) { const s = size(); api?.resize(s.width, s.height) } }, 250)
}).observe(desktop)
window.addEventListener('pagehide', () => { password.value = ''; token = ''; api?.shutdown() })

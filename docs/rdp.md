# RDP 第一版

OpsUp 使用 IronRDP WASM 在浏览器内运行远程桌面，Go 内置 RDCleanPath 网关负责目标连接、X.224 协商、TLS 和 WebSocket 转发。无需 guacd、FreeRDP 或额外守护进程；前端及 WASM 随 Go 二进制内嵌。

## 使用

1. 使用 HTTPS 访问 OpsUp（本机 localhost/127.0.0.1 开发调试除外）。反向代理需支持 `/api/rdp/` 的 WebSocket Upgrade，并设置足够长的会话超时。
2. 添加服务器，连接类型选择 RDP，填写主机、端口（默认 3389）、用户名和可选域。Windows 目标须开启远程桌面并支持 NLA，且允许该账户远程登录。
3. 若目标使用受系统信任的 CA 证书，证书指纹可留空，后端会验证证书链和主机名。自签名证书须填写从独立可信渠道取得的 **叶证书 SHA256 指纹**，64 位十六进制，可用冒号分隔。证书更新后需重新核验指纹。没有“忽略证书错误”选项。
4. 点击服务器，在 RDP 标签页中输入本次登录密码。关闭标签页或退出 OpsUp 会释放对应浏览器会话。
5. 工具栏提供断开、全屏、Ctrl+Alt+Del，以及手动双向剪贴板操作。浏览器可能要求剪贴板权限。调整窗口或切换回标签页时请求更新远程分辨率；目标不支持动态分辨率时仍可按画面比例缩放。

从 Windows 可信控制台获取正在使用的 RDP 证书 SHA256 指纹，可在 PowerShell 中运行：

```powershell
$rdpSettings = Get-CimInstance -Namespace root/cimv2/TerminalServices -ClassName Win32_TSGeneralSetting -Filter "TerminalName='RDP-tcp'"
$rdpCertificate = Get-ChildItem 'Cert:\LocalMachine\Remote Desktop', 'Cert:\LocalMachine\My' | Where-Object Thumbprint -eq $rdpSettings.SSLCertificateSHA1Hash | Select-Object -First 1
if (-not $rdpCertificate) { throw '未找到 RDP 使用的证书，请在证书管理器中核对' }
$rdpHasher = [System.Security.Cryptography.SHA256]::Create()
try { ([BitConverter]::ToString($rdpHasher.ComputeHash($rdpCertificate.RawData))).Replace('-', '').ToLowerInvariant() } finally { $rdpHasher.Dispose() }
```

Windows 设置中的 SHA1 thumbprint 仅用于定位证书，不能直接填入 OpsUp 的 SHA256 字段。

## 范围与凭据

- 支持多个不同服务器的 RDP 标签页，以及与 SSH 标签页并存；每个桌面运行在独立同源 iframe 中，避免上游组件全局状态和剪贴板互相干扰。
- 登录密码只在浏览器会话内存中使用，不提交给服务器管理 API，不存 SQLite、localStorage 或日志。浏览器仍需接触密码，Go TLS 网关仍处于受信任会话链路中。
- JWT 在 RDCleanPath 首帧中传输，不放入 URL。后端先验证 JWT，再按服务器 ID 查询目标；客户端提供的 destination 不决定实际拨号地址。
- TLS 最低为 1.2，并要求 NLA；不支持旧版纯 RDP 安全模式。
- RDP 记录拒绝保存密码、SSH 私钥、密钥复用、SSH 指纹和跳板机设置。SSH 转为 RDP 会清除原凭据。旧数据库记录迁移后仍为 SSH。
- 不提供音频、打印机、磁盘映射、文件传输、USB、SSH 跳板或 Kerberos KDC 代理配置。域名字段可用于 NTLM/NLA，不代表所有仅 Kerberos 的域环境均可连接。
- 剪贴板采用用户主动操作，不在后台自动同步。RDP 目标不显示 SFTP 功能。

## 构建与验证

构建命令不变：`cd ui && npm ci && npm run build`，随后在仓库根目录执行 `CGO_ENABLED=1 go build .`。Vite 同时生成 `index.html` 和 `rdp.html`。依赖锁定为 `@devolutions/iron-remote-desktop@0.11.0`、`@devolutions/iron-remote-desktop-rdp@0.7.0`（MIT OR Apache-2.0）。WASM 随 npm 包嵌入 JS，首次打开 RDP 时加载约 6 MB 未压缩资源；SSH 页面不加载该资源。

自动化覆盖 DER 线格式、NLA 要求、证书验证、无效 JWT、跨源升级拒绝、保存目标绑定、TLS 双向转发、断线资源回收、迁移与凭据清除、表单和 iframe 消息来源校验。

发布前仍需真实 Windows 验收：Windows 10/11 和 Server 2016/2019/2022/2025 的 NLA 登录、错误密码、键鼠、中文剪贴板、全屏/缩放、多标签切换、断线重连、证书轮换及长时间空闲。自动化中的模拟 TLS 服务器不等于完整 Windows RDP 兼容性测试。

上游协议和客户端参考：

- https://github.com/Devolutions/IronRDP/tree/master/crates/ironrdp-rdcleanpath
- https://github.com/Devolutions/IronRDP/tree/master/web-client/iron-remote-desktop

# OpsUp - Web SSH Terminal

[English](README.md)

基于浏览器的 SSH 终端管理工具，支持多服务器连接、跳板机代理、密钥复用。

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.25, Gin, gorilla/websocket, golang.org/x/crypto/ssh |
| 前端 | React 19, TypeScript, Ant Design 6, xterm.js 6, Vite 8 |
| 数据库 | SQLite 3 (WAL mode) |
| 部署 | 单二进制文件 (Go embed 嵌入前端) |

## 项目结构

```
OpsUp/
├── main.go                          # 入口：加载配置 → 初始化数据库 → 启动 HTTP 服务
├── go.mod / go.sum
├── Makefile                         # 构建：init, build, dev, clean
│
├── internal/                        # ──── Go 后端 ────
│   ├── config/
│   │   └── config.go                # 配置加载（环境变量：DB路径、JWT密钥、加密密钥、监听端口）
│   │
│   ├── database/
│   │   ├── database.go              # SQLite 初始化、WAL 模式、迁移执行
│   │   └── migrations.go            # DDL 建表语句 + 增量 ALTER TABLE 迁移
│   │
│   ├── models/
│   │   ├── user.go                  # User 模型：CountUsers, CreateUser, GetUserByUsername
│   │   └── server.go                # Server 模型：CRUD + ToListItem（隐藏私钥）
│   │
│   ├── auth/
│   │   ├── auth.go                  # bcrypt 密码哈希、JWT 签发/验证
│   │   └── middleware.go            # Gin 中间件：从 Authorization header 提取并验证 JWT
│   │
│   ├── crypto/
│   │   └── encryption.go            # AES-256-GCM 加解密（私钥加密存储）
│   │
│   ├── sshclient/
│   │   ├── client.go                # SSH 直连 + 跳板机连接 (DialViaJump)
│   │   └── session.go               # SSH Session：PTY、Shell、Resize、Stdin/Stdout pipe
│   │
│   ├── terminal/
│   │   └── manager.go               # 活跃终端会话注册表（sync.Mutex）
│   │
│   └── api/
│       ├── router.go                # 路由注册：公开(auth) / 受保护(servers) / WebSocket(terminal) / SPA
│       ├── auth_handler.go          # POST /auth/setup, /auth/login, GET /auth/status
│       ├── server_handler.go        # CRUD /servers + 密钥复用(copy_key_from) + 跳板机
│       └── terminal_handler.go      # WS /terminal/:id → SSH 连接 + 双向数据泵 + keepalive
│
└── ui/                              # ──── React 前端 ────
    ├── index.html                   # Vite 入口 HTML
    ├── package.json                 # 依赖：react, antd, @xterm/xterm, axios, react-router-dom
    ├── vite.config.ts               # Vite 配置：API 代理、构建输出目录
    ├── tsconfig.json
    │
    └── src/
        ├── main.tsx                 # ReactDOM 挂载
        ├── App.tsx                  # 路由：/login, /setup, /*（受保护）
        │
        ├── api/
        │   ├── client.ts            # Axios 实例：JWT 拦截器、401 自动跳转
        │   ├── auth.ts              # getAuthStatus, setup, login
        │   └── servers.ts           # listServers, createServer, updateServer, deleteServer, testConnection
        │
        ├── hooks/
        │   └── useAuth.ts           # 认证状态管理：login, setup, logout, token 持久化
        │
        ├── types/
        │   └── index.ts             # TypeScript 接口：Server, ServerForm, AuthStatus, LoginResponse
        │
        ├── pages/
        │   ├── LoginPage.tsx        # 登录页：检查初始化状态 → 登录表单
        │   ├── SetupPage.tsx         # 初始化页：首次创建管理员
        │   └── MainPage.tsx         # 主页：左侧栏 + 终端区域，管理 Tab 状态
        │
        ├── components/
        │   ├── ServerSidebar.tsx     # 服务器列表：分组折叠、搜索、右键菜单、增删改查
        │   ├── ServerFormModal.tsx   # 服务器表单：密钥粘贴/复用/文件导入、跳板机选择、分组
        │   ├── TerminalTabs.tsx      # 自定义 Tab 栏 + 字体大小调节
        │   └── TerminalTab.tsx       # xterm.js 终端：WebSocket 连接、resize、断线重连
        │
        └── styles/
            ├── global.css           # 全局样式、滚动条、Ant Design 覆盖
            └── terminal.css         # xterm 容器样式
```

## 数据库设计

```sql
-- 用户表
users (id, username UNIQUE, password bcrypt, created_at, updated_at)

-- 服务器表
servers (
    id, name, host, port DEFAULT 22, username,
    private_key AES-256-GCM加密,   -- 私钥加密存储
    description, group_name,        -- 分组
    jump_server_id DEFAULT NULL,    -- 跳板机（自引用）
    created_at, updated_at
)

-- 会话审计
sessions (id, user_id, server_id, started_at, ended_at, client_ip)
```

## API

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/auth/status` | 无 | 是否已初始化 |
| POST | `/api/auth/setup` | 无 | 首次创建管理员 |
| POST | `/api/auth/login` | 无 | 登录，返回 JWT |
| GET | `/api/servers` | JWT | 列出服务器 |
| POST | `/api/servers` | JWT | 添加服务器 |
| GET | `/api/servers/:id` | JWT | 服务器详情 |
| PUT | `/api/servers/:id` | JWT | 更新服务器 |
| DELETE | `/api/servers/:id` | JWT | 删除服务器 |
| POST | `/api/servers/:id/test` | JWT | 测试连接 |
| WS | `/api/terminal/:serverId?token=` | JWT | SSH 终端 |

## WebSocket 协议

二进制帧，首字节为类型标识：

| 字节 | 方向 | 说明 |
|------|------|------|
| `0x00` | 双向 | stdin/stdout 数据 |
| `0x01` | 双向 | resize(JSON)/stderr |
| `0x02` | 服务端→客户端 | 错误消息 |
| `0x03` | 服务端→客户端 | 状态变更 (connected/disconnected) |
| `0x04` | 服务端→客户端 | 连接确认 |

## 快速开始

需要 Go 1.25+、C 编译器（SQLite 使用 CGO）、make 和 Node.js 22.22.2+（或 24.15+）。

```bash
make build
# 先在受信任网络或本机完成管理员初始化，再通过 HTTPS 反向代理开放访问
OPSUP_LISTEN=127.0.0.1:8080 OPSUP_DB_PATH=./data/opsup.db ./opsup
```

开发时先运行 `make build-frontend`，然后在两个终端分别运行：

```bash
cd ui && npm run dev
# 在项目根目录；显式指定数据库，避免 go run 的临时可执行文件目录
OPSUP_DB_PATH="$PWD/dev-data/opsup.db" go run .
```

Vite 的 `/api` 代理同时支持 HTTP 和 WebSocket。

### 配置与备份

| 变量 | 默认值 / 要求 |
|---|---|
| `OPSUP_DB_PATH` | 可执行文件旁的 `opsup.db` |
| `OPSUP_LISTEN` | `:8080` |
| `OPSUP_SECRETS_PATH` | `<数据库路径>.secrets.json`，自动生成并以 `0600` 权限保存 |
| `OPSUP_JWT_SECRET` | 不设置则使用持久化的随机密钥；自定义值至少 32 字节，不得使用旧默认值 |
| `OPSUP_ENCRYPTION_KEY` | 不设置则使用持久化的随机密钥；自定义值必须是 64 位十六进制 |

**数据库与加密密钥必须一起备份。** 不要删除或每次启动重新生成密钥。通过环境变量配置的密钥应由部署系统持久保存；不要把密钥文件提交到 Git。两项密钥都由环境变量指定时，不会创建密钥文件。

Docker 的 `/data` 卷会同时保存默认数据库与密钥文件：

```bash
docker build -t opsup .
docker run -d -p 127.0.0.1:8080:8080 -v opsup-data:/data opsup
```

### 从旧版本升级

1. 停止旧服务，备份数据库（包括仍存在的 WAL 文件）和原有自定义加密密钥。
2. 原来使用自定义 `OPSUP_ENCRYPTION_KEY` 的部署必须继续提供同一个值。
3. 原来未配置密钥、使用旧公开默认值的部署，移除显式的旧默认值配置。新版本首次启动生成持久化随机密钥，并在事务中重新加密旧凭据。
4. 任何凭据无法解密时，升级会终止，不会部分改写凭据。恢复原始密钥后重试，不要删除数据库或密钥文件。
5. 默认 JWT 密钥会改变，需要重新登录；为每台服务器和跳板机配置可信主机指纹后才能连接。

### SSH 主机指纹

在服务器编辑表单中填入 `SHA256:…` 主机指纹。请通过可信控制台核对，例如：

```bash
for key in /etc/ssh/ssh_host_*_key.pub; do
  ssh-keygen -lf "$key"
done
```

首次连接未配置指纹时会拒绝连接，并显示观察到的指纹供核对。**不要仅凭失败提示就信任该指纹**。主机密钥变更也会拒绝连接，确认是合法更换后再更新配置。跳板机和目标机分别验证。

### SFTP 上传与连接限制

- 支持一个直连跳板机，不支持嵌套跳板链。
- 上传请求体上限 256 MiB（包含 multipart 开销），单次 SFTP 操作上限 15 分钟。
- 上传先写私有临时文件，再发布；覆盖现有普通文件时保留权限及 UID/GID，不能安全保留元数据时失败而不覆盖。新文件权限为 `0600`，不覆盖符号链接。
- 不支持 POSIX rename 扩展的服务器只允许上传新文件，拒绝覆盖。连接中断时远端可能残留 `.opsup-upload-*` 临时文件，确认没有进行中的上传后可清理。

## 验证与维护

```bash
make test                     # 前端类型检查、构建、测试；Go vet 与 race 测试
make clean                    # 只清理构建产物和前端依赖，不删除数据库或密钥
# 确认已停止服务并备份后，才执行显式的数据重置
make reset-data CONFIRM=DELETE DB_PATH=/absolute/path/to/opsup.db
```

GitHub Actions 检查前后端测试、Go 格式、类型、构建及 Docker 构建。

## 安全边界

- 私钥与 SSH 密码使用 AES-256-GCM 存储，API 不返回凭据或凭据密文；管理员登录密码使用 bcrypt。
- 新管理员密码至少 12 个字符，最多 72 个 UTF-8 字节。初始化采用原子写入，防止并发创建多个首位管理员。
- 登录与初始化按来源 IP 限流。默认不信任代理传入的来源 IP 头；代理后面的客户端会共享该代理 IP 的额度。
- JWT 有效期 24 小时。WebSocket 仍通过查询参数认证，应用访问日志不记录查询参数；**反向代理也必须过滤 token，或不要记录终端请求的完整 URL**。
- 应用按同源方式使用，不开放通配 CORS。请使用 HTTPS，不要把尚未初始化的实例直接暴露到公网。
- 当前仍是共享管理员的服务器管理工具，不提供多租户隔离；SFTP 可访问范围由远端 SSH 账号权限决定。

## RDP 远程桌面（第一版）

新增 IronRDP WASM 远程桌面，保持单二进制部署，密码每次连接时输入、不保存。配置、证书指纹和兼容性验收说明见 [RDP 使用文档](docs/rdp.md)。

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

```bash
# 构建
make build

# 运行（默认端口 8080）
./opsup

# 自定义配置
OPSUP_LISTEN=:9090 \
OPSUP_JWT_SECRET=your-secret \
OPSUP_ENCRYPTION_KEY=0123456789abcdef...（64位hex） \
./opsup

# 开发模式（前后端分别启动）
cd ui && npm run dev    # 前端 localhost:5173
go run main.go          # 后端 localhost:8080
```

## 安全设计

- **私钥加密存储**：AES-256-GCM，密钥从环境变量加载
- **密码哈希**：bcrypt
- **JWT 认证**：24 小时过期，WebSocket 通过 query param 认证
- **SQL 注入防护**：全程参数化查询
- **WebSocket 并发安全**：写操作通过 sync.Mutex 序列化
- **前端不暴露私钥**：密钥复用在后端直接拷贝加密数据，不经浏览器

# ZeroGo - P2P VPN Mesh Network

[![Go Version](https://img.shields.io/badge/Go-1.24-blue)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

ZeroGo是一个基于Go语言实现的P2P VPN mesh网络，提供了去中心化的虚拟网络解决方案。通过WebRTC技术实现NAT穿透，支持多平台部署，并提供现代化的Web管理界面。

## 特性

- 🔥 **P2P Mesh网络** - 去中心化网状网络拓扑
- 🌐 **NAT穿透** - 基于WebRTC/ICE的自动NAT穿透
- 🔒 **加密通信** - Noise Protocol加密协议
- 📡 **中继支持** - TURN中继服务器支持
- 🎨 **现代化UI** - React + TypeScript + Ant Design控制面板
- 🐳 **Docker支持** - 容器化部署，开箱即用
- 📱 **多平台** - 支持Linux、Windows、macOS、OpenWrt等

## 架构

ZeroGo由以下核心组件组成：

### zerogo-agent
VPN节点代理，负责：
- 创建虚拟网络设备（TAP）
- 与其他节点建立P2P连接
- 处理网络数据包转发
- NAT穿透和连接管理

### zerogo-controller
中央控制器，提供：
- RESTful API管理接口
- WebSocket实时通信
- 节点身份认证（JWT）
- 网络状态监控

### zerogo-relay
中继服务器，用于：
- TURN协议中继
- 辅助NAT穿透
- 连接失败时的备用路由

### zerogo-cli
命令行工具，用于：
- 网络配置管理
- 节点状态查询
- 调试和诊断

### Web控制面板
现代化的Web界面：
- React 18 + TypeScript
- Ant Design 5 UI组件
- Framer Motion动画
- Recharts数据可视化

## 快速开始

### 前置要求

- Go 1.24+
- Node.js 18+ (仅构建Web界面时需要)
- Docker (可选)

### 使用Makefile构建

```bash
# 构建所有组件
make build

# 构建特定组件
make agent      # 构建zerogo-agent
make controller # 构建zerogo-controller
make relay      # 构建zerogo-relay
make cli        # 构建zerogo-cli

# 运行测试
make test

# 代码检查
make lint

# 清理构建产物
make clean
```

### 使用Docker构建

```bash
# 构建镜像
docker build -t zerogo:latest .

# 运行agent
docker run --privileged --network host zerogo:latest

# 运行controller
docker run -p 9394:9394 -v $(pwd)/data:/var/lib/zerogo zerogo:latest zerogo-controller
```

### Web界面开发

```bash
cd web

# 安装依赖
npm install

# 开发模式
npm run dev

# 构建生产版本
npm run build

# 预览生产构建
npm run preview
```

## 部署

### Controller部署

```bash
# 启动controller
./bin/zerogo-controller -config configs/controller.yaml

# 或使用Docker Compose
docker-compose up -d controller
```

### Agent部署

```bash
# 连接到controller
./bin/zerogo-agent -controller http://controller:9394 -token <your-token>

# 或使用配置文件
./bin/zerogo-agent -config /etc/zerogo/agent.yaml
```

### Relay部署

```bash
# 启动中继服务
./bin/zerogo-relay -listen :3478
```

## 配置

### Controller配置示例

```yaml
# configs/controller.yaml
listen: 0.0.0.0:9394
database: data/controller.db
jwt-secret: your-secret-key
log-level: info
```

### Agent配置示例

```yaml
# /etc/zerogo/agent.yaml
controller: https://controller.example.com
token: your-jwt-token
tap-device: zerogo0
log-level: info
```

## Web界面

访问 `http://localhost:5173` 打开Web控制面板（开发模式）

主要功能：
- 📊 网络拓扑可视化
- 📈 流量监控和统计
- 🔗 节点连接管理
- ⚙️ 网络配置设置
- 🎯 实时状态更新

## 技术栈

### 后端
- **Go 1.24** - 核心语言
- **Gin** - Web框架
- **GORM** - ORM
- **SQLite** - 数据库
- **Pion WebRTC** - NAT穿透
- **Water** - TAP设备管理
- **Noise Protocol** - 加密通信

### 前端
- **React 18** - UI框架
- **TypeScript** - 类型安全
- **Ant Design 5** - UI组件库
- **Framer Motion** - 动画
- **Recharts** - 图表
- **Axios** - HTTP客户端
- **Vite** - 构建工具

## 交叉编译

```bash
# Linux AMD64
make build-linux-amd64

# Linux ARM64
make build-linux-arm64

# Windows
make build-windows

# OpenWrt MIPS
make build-openwrt-mips
```

## 开发指南

### 目录结构

```
zerogo/
├── cmd/               # 主程序入口
│   ├── zerogo-agent/
│   ├── zerogo-controller/
│   ├── zerogo-relay/
│   └── zerogo-cli/
├── internal/          # 内部包
│   ├── agent/        # Agent逻辑
│   ├── controller/   # Controller逻辑
│   ├── relay/        # Relay逻辑
│   ├── vl1/          # 虚拟层1（传输层）
│   ├── vl2/          # 虚拟层2（网络层）
│   ├── tap/          # TAP设备
│   └── identity/     # 身份管理
├── web/              # Web前端
├── configs/          # 配置文件
├── data/             # 运行时数据
├── Makefile          # 构建脚本
├── Dockerfile        # Docker镜像
└── go.mod            # Go模块
```

### 贡献指南

1. Fork本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启Pull Request

## 测试

```bash
# 运行所有测试
make test

# 运行特定包测试
go test ./internal/agent/...

# 运行测试并查看覆盖率
go test -cover ./...
```

## 许可证

本项目采用MIT许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 致谢

- [ZeroTier](https://www.zerotier.com/) - 灵感来源
- [Pion WebRTC](https://github.com/pion/webrtc) - WebRTC实现
- [Ant Design](https://ant.design/) - UI组件库

## 联系方式

- 项目主页: [GitHub](https://github.com/unicornultrafoundation/zerogo)
- 问题反馈: [Issues](https://github.com/unicornultrafoundation/zerogo/issues)

---

**注意**: 本项目目前处于开发阶段，不建议用于生产环境。

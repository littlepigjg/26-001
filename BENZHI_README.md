# 基于 Go 实现的配置中心服务，一款轻量级配置管理工具，完成多应用多环境的配置项管理、版本控制、变更审计与客户端配置推送功能

## 项目简介

配置中心（Configuration Center）是一个轻量级的配置管理服务，采用纯 Go 标准库开发，无需任何第三方依赖。它提供了完整的配置生命周期管理能力：

- **多应用管理**：支持创建和管理多个应用（App）
- **多环境支持**：每个应用支持 dev、test、staging、prod 等多个环境
- **配置 CRUD**：完整的 Key-Value 配置项增删改查
- **版本历史**：自动记录配置变更的版本历史，支持版本对比
- **客户端拉取**：基于版本号的配置推送，支持 ETag/304 优化
- **变更审计**：所有操作自动记录审计日志
- **回滚支持**：一键回滚到任意历史版本
- **配置校验**：支持 JSON、YAML、Number、Boolean、URL、Email 等多种格式校验

## 目录结构

```
config-center/
├── cmd/server/main.go          # 入口文件
├── internal/
│   ├── config/                  # 配置加载
│   ├── handler/                 # HTTP 处理器
│   ├── middleware/              # 中间件
│   ├── model/                   # 数据模型
│   ├── router/                  # 路由配置
│   ├── service/                 # 业务逻辑
│   └── store/                   # 存储层
├── pkg/
│   ├── cache/                   # 缓存实现
│   ├── concurrent/              # 并发工具
│   ├── diff/                    # 差异比较
│   ├── envutil/                 # 环境变量工具
│   ├── hash/                    # 哈希工具
│   ├── lock/                    # 分布式锁
│   ├── logger/                  # 结构化日志
│   ├── response/                # 统一响应格式
│   ├── retry/                   # 重试工具
│   └── stringutil/              # 字符串工具
├── web/                         # 前端静态文件
├── benzhi.Dockerfile            # Docker 构建文件
├── build_benzhi_docker.sh       # Docker 构建脚本
├── BUG_CATALOG.md               # 缺陷候选清单
└── go.mod                       # Go 模块定义
```

## API 文档

### 健康检查

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/health` | 健康检查，返回运行时状态 |
| GET | `/ready` | 就绪检查，返回各组件状态 |

**Health 响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "healthy",
    "timestamp": "2026-08-25T12:00:00Z",
    "uptime": 120.5,
    "go_version": "go1.22.0",
    "num_goroutines": 5,
    "memory_usage": {
      "alloc": 1048576,
      "total_alloc": 2097152,
      "sys": 12582912,
      "num_gc": 0
    }
  }
}
```

### 应用管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/apps` | 获取应用列表（支持分页） |
| POST | `/api/apps` | 创建新应用 |
| GET | `/api/apps/{id}` | 获取单个应用详情 |
| PUT | `/api/apps/{id}` | 更新应用信息 |
| DELETE | `/api/apps/{id}` | 删除应用 |

**创建应用请求：**
```json
{
  "name": "my-app",
  "description": "My application",
  "owner": "dev-team"
}
```

### 配置管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/configs?app_id={app}&environment={env}` | 获取配置列表 |
| POST | `/api/configs` | 创建新配置项 |
| GET | `/api/configs/{app}/{env}/{key}` | 获取单个配置项 |
| PUT | `/api/configs/{app}/{env}/{key}` | 更新配置项 |
| DELETE | `/api/configs/{app}/{env}/{key}` | 删除配置项 |

**创建配置请求：**
```json
{
  "app_id": "my-app",
  "environment": "dev",
  "key": "database.host",
  "value": "localhost:5432",
  "description": "Database connection host",
  "format": "string"
}
```

### 版本管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/versions?app_id={app}&environment={env}` | 获取版本历史 |
| POST | `/api/versions` | 创建新版本快照 |
| GET | `/api/versions/{app}/{env}/{version}` | 获取指定版本详情 |

### 客户端拉取

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/client/pull?app_id={app}&environment={env}&version={hash}` | 拉取配置（支持版本判断） |
| POST | `/api/client/batch-pull` | 批量拉取多个应用配置 |

**批量拉取请求：**
```json
{
  "requests": [
    {"app_id": "app1", "environment": "dev", "version": "abc123"},
    {"app_id": "app2", "environment": "prod", "version": "def456"}
  ]
}
```

### 回滚

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/rollback` | 回滚到指定版本 |

**回滚请求：**
```json
{
  "app_id": "my-app",
  "environment": "dev",
  "target_version": 3
}
```

### 校验

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/validate` | 校验配置格式和完整性 |

### 差异比较

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/diff?app_id={app}&environment={env}&v1={n}&v2={n}` | 比较两个版本的差异 |

### 审计日志

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/audit-logs?app_id={app}&action={type}` | 查询操作审计日志 |

## 本地运行

```bash
# 克隆项目后进入目录
cd config-center

# 编译运行
go run ./cmd/server

# 指定配置文件
go run ./cmd/server -config config.json

# 服务启动后访问
curl http://localhost:8080/health
```

服务器默认监听 `0.0.0.0:8080`。

## Docker 构建和运行

```bash
# 使用默认参数构建
./build_benzhi_docker.sh

# 自定义镜像名和标签
./build_benzhi_docker.sh my-config-center v1.0.0

# 运行容器
docker run -d -p 8080:8080 --name config-center config-center:latest

# 查看日志
docker logs -f config-center

# 停止容器
docker stop config-center
docker rm config-center
```

## 测试命令

```bash
# 完整测试套件
go test -race -count=1 ./...

# 特定包测试
go test -v -race ./internal/service/...

# 基准测试
go test -bench=. ./...

# 代码覆盖率
go test -cover ./...

# 静态分析
go vet ./...

# 运行时检查
go run -race ./cmd/server
```

## 技术特性

- **纯 Go 标准库**：无任何第三方依赖
- **优雅关闭**：监听 SIGINT/SIGTERM 信号
- **健康检查**：/health 和 /ready 两个端点
- **结构化日志**：支持多级别、带上下文的日志
- **参数校验**：完善的请求参数验证
- **并发安全**：所有数据操作使用互斥锁保护
- **版本缓存**：基于内容哈希的配置缓存
- **CORS 支持**：跨域资源共享支持
- **请求恢复**：自动 panic recovery

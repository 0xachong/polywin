# PolyWin - Windows 热更新守护程序

一个使用 Golang 开发的 Windows 热更新守护程序，支持从 Git 仓库自动检测更新、构建新版本并重启目标程序。

## 架构说明

项目包含两个独立的程序：

1. **polywin.exe** - 守护程序（热更新管理器）
   - 监控 Git 仓库更新
   - 自动构建新版本
   - 管理目标程序的启动、停止和重启

2. **server.exe** - HTTP 服务器（被更新的目标程序）
   - Gin HTTP 服务，端口 8099
   - 提供 `/ping` API 接口
   - 这是热更新的目标程序

## 功能特性

- ✅ 自动检测 Git 仓库代码更新
- ✅ 自动构建新版本（从 Git 仓库）
- ✅ 自动重启目标程序以应用更新
- ✅ Windows 平台文件替换和重启
- ✅ 守护程序监控目标程序运行状态
- ✅ 可配置的更新检查间隔

## 编译和运行

### 1. 安装依赖

```bash
go mod download
```

### 2. 编译 Windows 可执行文件

使用提供的构建脚本：

```bash
chmod +x build.sh
./build.sh
```

或者手动编译：

```bash
# 编译守护程序
GOOS=windows GOARCH=amd64 go build -o polywin.exe main.go updater.go

# 编译服务器程序
GOOS=windows GOARCH=amd64 go build -o server.exe server.go
```

### 3. 运行程序

将两个 exe 文件复制到 Windows 机器上，然后运行：

```bash
# 启动守护程序（会自动启动 server.exe）
polywin.exe -repo=https://github.com/0xachong/polywin.git -check-interval=5m

# 或者使用 SSH 格式的 Git URL
polywin.exe -repo=git@github.com:0xachong/polywin.git -check-interval=5m
```

## 命令行参数

- `-repo`: Git 仓库 URL（默认：https://github.com/0xachong/polywin.git）
- `-target`: 目标可执行文件名（默认：server.exe）
- `-check-interval`: 更新检查间隔（默认：5分钟）
- `-auto-update`: 是否启用自动更新（默认：true）

## API 接口

服务器程序（server.exe）提供以下 HTTP 接口：

- `GET /ping` - Ping/Pong 健康检查
  ```json
  {
    "message": "pong",
    "version": "1.0.0",
    "time": "2024-12-25 15:30:00"
  }
  ```

- `GET /version` - 版本信息
  ```json
  {
    "version": "1.0.0",
    "port": "8099"
  }
  ```

- `GET /` - 服务状态
  ```json
  {
    "service": "polywin-http-server",
    "version": "1.0.0",
    "status": "running"
  }
  ```

## 工作原理

1. **守护程序启动**：
   - polywin.exe 启动后，检查 server.exe 是否存在
   - 如果不存在，尝试构建 server.exe
   - 启动 server.exe 并监控其运行状态

2. **更新检测**：
   - 定期克隆 Git 仓库并检查最新提交哈希
   - 如果发现新提交，触发更新流程

3. **构建新版本**：
   - 克隆仓库到临时目录
   - 构建 server.go 生成新的 server.exe.new
   - 等待当前 server.exe 退出后替换文件

4. **自动重启**：
   - 守护程序监控 server.exe 进程
   - 如果进程退出，自动重启
   - 重启时会使用新版本（如果已更新）

## 目录结构

```
polywin/
├── main.go          # 守护程序入口
├── updater.go       # 更新器实现
├── server.go        # HTTP 服务器（目标程序）
├── build.sh         # 构建脚本
├── go.mod           # Go 模块定义
├── go.sum           # 依赖校验和
└── README.md        # 说明文档
```

## 测试步骤

1. **本地测试**：
   ```bash
   # 在 Windows 上运行
   polywin.exe
   
   # 测试 API
   curl http://localhost:8099/ping
   ```

2. **测试热更新**：
   - 修改 server.go 中的代码（例如修改版本号）
   - 提交并推送到 Git 仓库
   - 等待守护程序检测到更新（默认 5 分钟）
   - 观察 server.exe 是否自动重启并应用新版本

## 注意事项

1. **文件锁定**：在 Windows 上，正在运行的可执行文件无法直接替换，程序使用批处理脚本在退出后执行替换操作。

2. **构建要求**：目标机器需要安装 Go 编译器和 Git，以便从 Git 仓库构建新版本。

3. **权限要求**：程序需要对可执行文件所在目录有写入权限。

4. **网络要求**：需要能够访问 Git 仓库（HTTPS 或 SSH）。

5. **Git 仓库格式**：
   - HTTPS: `https://github.com/user/repo.git`
   - SSH: `git@github.com:user/repo.git`
   - 简化格式: `github.com:user/repo.git`（会自动转换为 SSH 格式）

## 开发建议

1. 修改 `server.go` 来添加你的业务逻辑和 API 接口
2. 根据实际需求调整更新检查间隔
3. 可以在 server.go 中添加更多 API 接口
4. 建议在生产环境中使用 HTTPS 和签名验证

## 许可证

MIT License

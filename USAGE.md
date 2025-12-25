# PolyWin 使用指南

## 快速开始

### 1. 编译程序

在 macOS/Linux 上交叉编译 Windows 程序：

```bash
# 给构建脚本添加执行权限
chmod +x build.sh

# 执行构建
./build.sh
```

构建完成后会生成两个文件：
- `polywin.exe` - 守护程序（约 12MB）
- `server.exe` - HTTP 服务器（约 11MB）

### 2. 部署到 Windows

将两个 `.exe` 文件复制到 Windows 机器上（建议放在同一个目录）。

### 3. 运行程序

#### 方式一：直接运行（使用默认配置）

```cmd
polywin.exe
```

默认配置：
- Git 仓库：`https://github.com/0xachong/polywin.git`
- 更新检查间隔：5 分钟
- 目标程序：`server.exe`
- 自动更新：启用

#### 方式二：自定义配置

```cmd
# 指定 Git 仓库和检查间隔
polywin.exe -repo=https://github.com/0xachong/polywin.git -check-interval=1m

# 使用 SSH 格式的 Git URL
polywin.exe -repo=git@github.com:0xachong/polywin.git -check-interval=2m

# 禁用自动更新
polywin.exe -auto-update=false

# 指定不同的目标程序名
polywin.exe -target=myserver.exe
```

### 4. 配置公网访问

服务器默认监听 `0.0.0.0:8099`，已支持公网访问。如果需要自定义配置：

```cmd
# 设置监听地址和端口（环境变量）
set HOST=0.0.0.0
set PORT=8099
polywin.exe

# 或者只设置端口
set PORT=9099
polywin.exe
```

**重要提示**：
- 默认监听 `0.0.0.0`，支持所有网络接口访问
- 确保 Windows 防火墙允许 `8099` 端口入站连接
- 如果服务器在云服务器上，需要在安全组中开放相应端口

### 5. 测试 HTTP 服务

守护程序启动后会自动启动 `server.exe`，HTTP 服务监听在 `8099` 端口。

#### 使用 curl 测试

```bash
# Ping/Pong 接口
curl http://localhost:8099/ping

# 版本信息
curl http://localhost:8099/version

# 服务状态
curl http://localhost:8099/
```

#### 使用浏览器测试

**本地访问**：
- http://localhost:8099/ping
- http://localhost:8099/version
- http://localhost:8099/

**公网访问**（如果服务器有公网 IP）：
- http://<your-server-ip>:8099/ping
- http://<your-server-ip>:8099/version
- http://<your-server-ip>:8099/

#### 预期响应

**GET /ping**
```json
{
  "message": "pong",
  "version": "1.0.0",
  "time": "2024-12-25 15:30:00"
}
```

**GET /version**
```json
{
  "version": "1.0.0",
  "port": "8099"
}
```

**GET /**
```json
{
  "service": "polywin-http-server",
  "version": "1.0.0",
  "status": "running"
}
```

## 测试热更新功能

### 步骤 1：修改代码

编辑 `server.go`，例如修改版本号：

```go
var (
	serverVersion = "1.0.1"  // 从 1.0.0 改为 1.0.1
	serverPort    = "8099"
)
```

### 步骤 2：提交并推送到 Git

```bash
git add server.go
git commit -m "更新版本到 1.0.1"
git push origin main
```

### 步骤 3：等待自动更新

守护程序会：
1. 每 5 分钟（或你设置的间隔）检查一次 Git 仓库
2. 发现新提交后，自动克隆仓库
3. 构建新的 `server.exe.new`
4. 等待当前 `server.exe` 退出后替换文件
5. 自动重启 `server.exe`

### 步骤 4：验证更新

```bash
# 检查新版本
curl http://localhost:8099/version

# 应该返回新版本号
{
  "version": "1.0.1",
  "port": "8099"
}
```

## 命令行参数说明

| 参数 | 说明 | 默认值 | 示例 |
|------|------|--------|------|
| `-repo` | Git 仓库 URL | `https://github.com/0xachong/polywin.git` | `-repo=https://github.com/user/repo.git` |
| `-target` | 目标可执行文件名 | `server.exe` | `-target=myserver.exe` |
| `-check-interval` | 更新检查间隔 | `5m` | `-check-interval=1m` |
| `-auto-update` | 是否启用自动更新 | `true` | `-auto-update=false` |

### 时间间隔格式

支持的时间单位：
- `s` - 秒（如：`30s`）
- `m` - 分钟（如：`5m`）
- `h` - 小时（如：`1h`）

示例：
```cmd
polywin.exe -check-interval=30s   # 30 秒检查一次
polywin.exe -check-interval=2m    # 2 分钟检查一次
polywin.exe -check-interval=1h    # 1 小时检查一次
```

## 日志输出

### 守护程序日志

```
PolyWin 守护程序启动，版本: 1.0.0
目标程序: server.exe
Git 仓库: https://github.com/0xachong/polywin.git
更新检查间隔: 5m0s
启动服务器程序: C:\path\to\server.exe
服务器程序已启动，PID: 12345
自动更新检查已启动
正在检查更新...
当前已是最新版本
```

### 更新过程日志

```
正在检查更新...
发现新版本: a1b2c3d4，当前版本: 1.0.0
开始执行更新到版本: a1b2c3d4
开始构建新版本...
克隆仓库失败: ...
构建完成
更新脚本已启动，将在目标程序退出后自动替换文件
更新完成，等待服务器程序重启以应用更新
```

## 常见问题

### 1. server.exe 不存在怎么办？

守护程序会自动检测，如果 `server.exe` 不存在，会尝试自动构建。但需要：
- 当前目录有 `server.go` 文件
- 系统已安装 Go 编译器

### 2. 如何手动停止程序？

按 `Ctrl+C` 停止守护程序，守护程序会自动停止 `server.exe`。

### 3. 更新失败怎么办？

检查：
- Git 仓库 URL 是否正确
- 网络连接是否正常
- 是否有 Git 和 Go 编译器
- 是否有文件写入权限

### 4. 如何查看程序是否在运行？

```cmd
# Windows 上查看进程
tasklist | findstr polywin.exe
tasklist | findstr server.exe
```

### 5. 如何修改服务器端口和监听地址？

使用环境变量配置：

```cmd
# 修改端口
set PORT=9099
polywin.exe

# 修改监听地址（默认 0.0.0.0 支持公网访问）
set HOST=127.0.0.1  # 仅本地访问
set HOST=0.0.0.0    # 支持公网访问（默认）
polywin.exe

# 同时设置
set HOST=0.0.0.0
set PORT=9099
polywin.exe
```

### 6. 如何配置 Windows 防火墙？

允许公网访问需要开放防火墙端口：

```cmd
# 以管理员身份运行 PowerShell
netsh advfirewall firewall add rule name="PolyWin HTTP Server" dir=in action=allow protocol=TCP localport=8099
```

或者通过图形界面：
1. 打开"Windows Defender 防火墙"
2. 点击"高级设置"
3. 选择"入站规则" → "新建规则"
4. 选择"端口" → "TCP" → 输入端口 `8099`
5. 允许连接 → 应用到所有配置文件

## 生产环境建议

1. **使用 HTTPS**：在生产环境中，建议使用 HTTPS 访问 API
2. **设置防火墙**：只开放必要的端口（8099）
3. **日志管理**：将日志输出到文件，便于排查问题
4. **监控告警**：添加监控系统，检测服务是否正常运行
5. **版本管理**：使用 Git 标签管理版本，而不是提交哈希

## 示例：完整工作流程

```bash
# 1. 编译
./build.sh

# 2. 复制到 Windows 机器
scp polywin.exe server.exe user@windows-machine:/path/to/app/

# 3. 在 Windows 上运行
cd /path/to/app
polywin.exe -repo=https://github.com/0xachong/polywin.git -check-interval=5m

# 4. 测试服务
curl http://localhost:8099/ping

# 5. 修改代码并推送
# ... 编辑 server.go ...
git add server.go
git commit -m "更新功能"
git push

# 6. 等待自动更新（最多 5 分钟）
# 7. 验证更新
curl http://localhost:8099/version
```

## 故障排查

### 查看详细日志

如果遇到问题，可以查看控制台输出的详细日志信息。

### 检查文件

```cmd
# 检查文件是否存在
dir polywin.exe
dir server.exe

# 检查是否有 .new 或 .old 文件（更新过程中的临时文件）
dir *.new
dir *.old
```

### 手动清理

如果更新过程中断，可能需要手动清理：

```cmd
# 删除临时文件
del server.exe.new
del server.exe.old
del update_server.bat
```

然后重新启动 `polywin.exe`。


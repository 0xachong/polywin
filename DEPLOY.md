# 部署说明

## Windows 一键部署

### 方式一：直接双击运行（推荐）

1. **编译程序**（在 macOS/Linux 上）：
   ```bash
   ./build.sh
   ```

2. **复制文件到 Windows**：
   - 将 `polywin.exe` 复制到 Windows 机器
   - （可选）将 `start.bat` 也复制过去，方便双击启动

3. **双击运行**：
   - 直接双击 `polywin.exe` 或 `start.bat`
   - 程序会自动：
     - 检查 `server.exe` 是否存在
     - 如果不存在，尝试自动构建（需要 Go 环境）
     - 启动 HTTP 服务器（端口 8099）
     - 开始监控 Git 仓库更新

### 方式二：包含 server.exe 一起部署

如果你已经编译好了 `server.exe`：

1. 将 `polywin.exe` 和 `server.exe` 放在同一目录
2. 双击 `polywin.exe` 即可

## 配置说明

所有配置已硬编码，无需命令行参数：

- **Git 仓库**: `https://github.com/0xachong/polywin.git`
- **目标程序**: `server.exe`
- **更新检查间隔**: 5 分钟
- **自动更新**: 启用

## 首次运行

如果 Windows 机器上没有 `server.exe`，程序会尝试自动构建：

**前提条件**：
- 已安装 Go 编译器
- 已安装 Git
- 当前目录或可执行文件目录有 `server.go` 文件

**如果构建失败**：
- 手动编译 `server.exe` 并放在同一目录
- 或从 GitHub 下载预编译的 `server.exe`

## 测试

启动后，访问：
- http://localhost:8099/ping
- http://localhost:8099/version

## 注意事项

1. **防火墙**：确保 Windows 防火墙允许 8099 端口
2. **网络**：需要能访问 GitHub（用于检查更新）
3. **权限**：程序需要对目录有写入权限（用于更新文件）


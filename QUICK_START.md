# 快速开始指南

## ❌ 只拷贝 polywin.exe 可以吗？

**不可以**。只拷贝 `polywin.exe` 无法直接运行，因为：

1. **缺少 server.exe**：程序需要 `server.exe` 作为目标程序
2. **自动构建需要环境**：如果 `server.exe` 不存在，程序会尝试自动构建，但需要：
   - Go 编译器已安装
   - Git 已安装
   - `server.go` 文件存在

## ✅ 正确的部署方式

### 方式一：同时拷贝两个文件（推荐）

1. **下载两个文件**：
   - `polywin.exe` - 守护程序
   - `server.exe` - HTTP 服务器

2. **放在同一目录**：
   ```
   C:\polywin\
   ├── polywin.exe
   └── server.exe
   ```

3. **双击运行**：
   - 直接双击 `polywin.exe` 即可

### 方式二：从 GitHub Releases 下载

1. 访问：https://github.com/0xachong/polywin/releases
2. 下载最新版本的 `polywin.exe` 和 `server.exe`
3. 放在同一目录，双击运行

### 方式三：从 releases 目录下载

1. 访问：https://github.com/0xachong/polywin/tree/main/releases
2. 下载 `polywin.exe` 和 `server.exe`
3. 放在同一目录，双击运行

## 🔄 热更新工作流程

一旦 `polywin.exe` 和 `server.exe` 都部署好了：

1. **启动**：双击 `polywin.exe`
   - 自动启动 `server.exe`
   - HTTP 服务运行在 8099 端口

2. **自动更新**：
   - 每 5 分钟检查一次 GitHub 仓库
   - 发现新代码后自动：
     - 克隆仓库
     - 构建新的 `server.exe`
     - 等待当前 `server.exe` 退出
     - 替换文件并重启

3. **无需手动操作**：整个过程完全自动化

## 📋 部署检查清单

- [ ] 已下载 `polywin.exe`
- [ ] 已下载 `server.exe`
- [ ] 两个文件放在同一目录
- [ ] Windows 防火墙允许 8099 端口（如需公网访问）
- [ ] 网络可以访问 GitHub（用于检查更新）
- [ ] Windows 机器有 Go 编译器（用于自动构建新版本，可选）

## ⚠️ 重要提示

### 热更新需要 Go 编译器

虽然首次运行只需要两个 exe 文件，但**热更新功能需要 Go 编译器**：

- 当检测到 GitHub 有新代码时
- 程序会克隆仓库并构建新的 `server.exe`
- 这需要 Windows 机器上安装 Go 编译器

### 如果没有 Go 编译器

如果 Windows 机器没有 Go 编译器：
- 首次运行可以正常（使用已有的 `server.exe`）
- 但无法自动更新（构建会失败）
- 需要手动下载新版本的 `server.exe` 替换

## 🚀 最简单的部署

**推荐做法**：同时下载两个 exe 文件，放在同一目录，双击运行即可！


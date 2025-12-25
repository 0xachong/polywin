# PolyWin 预编译版本

本目录包含预编译的 Windows 可执行文件。

## 文件说明

- **polywin.exe** - 守护程序（热更新管理器）
- **server.exe** - HTTP 服务器（被更新的目标程序）

## 使用方法

1. 下载 `polywin.exe` 和 `server.exe`
2. 将两个文件放在同一目录
3. 双击 `polywin.exe` 即可运行

## 自动构建

这些文件由 GitHub Actions 自动构建：
- 每次推送到 `main` 分支时自动构建
- 创建 tag 时会自动发布到 Releases

## 最新版本

请查看 [GitHub Releases](https://github.com/0xachong/polywin/releases) 获取最新版本。


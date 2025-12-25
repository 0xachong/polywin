#!/bin/bash

# 构建脚本：分别编译守护程序和服务器程序

echo "开始构建 PolyWin 项目..."

# 构建守护程序 (polywin.exe) - 只编译 main.go 和 updater.go
# 注意：repo URL 已硬编码为 https://github.com/0xachong/polywin.git
echo "构建守护程序 (polywin.exe)..."
GOOS=windows GOARCH=amd64 go build -o polywin.exe -ldflags "-X main.version=1.0.0" main.go updater.go

if [ $? -eq 0 ]; then
    echo "✓ 守护程序构建成功: polywin.exe"
else
    echo "✗ 守护程序构建失败"
    exit 1
fi

# 构建服务器程序 (server.exe)
echo "构建服务器程序 (server.exe)..."
GOOS=windows GOARCH=amd64 go build -o server.exe -ldflags "-X main.serverVersion=1.0.0" ./cmd/server

if [ $? -eq 0 ]; then
    echo "✓ 服务器程序构建成功: server.exe"
else
    echo "✗ 服务器程序构建失败"
    exit 1
fi

echo ""
echo "构建完成！"
echo "  - polywin.exe: 守护程序（热更新管理器）"
echo "  - server.exe:  HTTP 服务器（被更新的目标程序）"


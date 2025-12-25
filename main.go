//go:build !server
// +build !server

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	version          = "1.0.0"
	// 硬编码的配置
	repoURL          = "https://github.com/0xachong/polywin.git"
	targetExecutable = "server.exe"
	checkInterval    = 5 * time.Minute
	enableAutoUpdate = true
)

var serverCmd *exec.Cmd

func main() {
	log.Printf("PolyWin 守护程序启动，版本: %s", version)
	log.Printf("目标程序: %s", targetExecutable)
	log.Printf("Git 仓库: %s", repoURL)
	log.Printf("更新检查间隔: %v", checkInterval)

	// 获取当前目录
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("获取可执行文件路径失败: %v", err)
	}
	execDir := filepath.Dir(execPath)
	targetPath := filepath.Join(execDir, targetExecutable)

	// 检查目标程序是否存在，不存在则尝试构建
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		log.Printf("目标程序 %s 不存在，尝试构建...", targetPath)
		if err := buildServer(execDir); err != nil {
			log.Printf("构建失败: %v，尝试从当前目录查找 server.go...", err)
			// 如果构建失败，尝试从当前目录构建
			wd, _ := os.Getwd()
			if err := buildServerFromDir(wd, execDir); err != nil {
				log.Fatalf("无法构建目标程序，请确保 server.go 文件存在: %v", err)
			}
		}
		log.Printf("目标程序构建成功: %s", targetPath)
	}

	// 创建更新器
	updater := NewUpdater(&UpdaterConfig{
		RepoURL:          repoURL,
		CheckInterval:    checkInterval,
		EnableAutoUpdate: enableAutoUpdate,
		CurrentVersion:   version,
		TargetExecutable: targetExecutable,
		TargetPath:       targetPath,
	})

	// 启动更新检查协程
	if enableAutoUpdate {
		go updater.StartUpdateChecker()
		log.Println("自动更新检查已启动")
	}

	// 启动目标程序
	startServer(targetPath)

	// 监控目标程序，如果退出则重启
	go monitorServer(targetPath, updater)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("程序正在关闭...")
	stopServer()
	updater.Stop()
	os.Exit(0)
}

// buildServer 构建服务器程序（从可执行文件所在目录查找 server.go）
func buildServer(targetDir string) error {
	log.Println("开始构建服务器程序...")

	// 首先尝试从可执行文件所在目录查找 server.go
	serverGoPath := filepath.Join(targetDir, "server.go")
	if _, err := os.Stat(serverGoPath); err == nil {
		return buildServerFromFile(serverGoPath, targetDir)
	}

	// 如果不存在，尝试从当前工作目录查找
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return buildServerFromDir(wd, targetDir)
}

// buildServerFromFile 从指定文件构建
func buildServerFromFile(serverGoPath, targetDir string) error {
	serverDir := filepath.Dir(serverGoPath)
	outputPath := filepath.Join(targetDir, "server.exe")
	
	buildCmd := exec.Command("go", "build", "-tags", "server", "-o", outputPath, "server.go")
	buildCmd.Dir = serverDir
	buildCmd.Env = os.Environ()

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("构建失败: %v, 输出: %s", err, string(output))
	}

	if len(output) > 0 {
		log.Printf("构建输出: %s", string(output))
	}

	log.Printf("服务器程序构建完成: %s", outputPath)
	return nil
}

// buildServerFromDir 从指定目录构建
func buildServerFromDir(sourceDir, targetDir string) error {
	serverGoPath := filepath.Join(sourceDir, "server.go")
	if _, err := os.Stat(serverGoPath); os.IsNotExist(err) {
		return fmt.Errorf("server.go 文件不存在于: %s", sourceDir)
	}
	return buildServerFromFile(serverGoPath, targetDir)
}

// startServer 启动服务器程序
func startServer(serverPath string) {
	log.Printf("启动服务器程序: %s", serverPath)

	serverCmd = exec.Command(serverPath)
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	serverCmd.Dir = filepath.Dir(serverPath)
	serverCmd.Env = os.Environ()

	if err := serverCmd.Start(); err != nil {
		log.Fatalf("启动服务器程序失败: %v", err)
	}

	log.Printf("服务器程序已启动，PID: %d", serverCmd.Process.Pid)
}

// stopServer 停止服务器程序
func stopServer() {
	if serverCmd != nil && serverCmd.Process != nil {
		log.Println("正在停止服务器程序...")
		if err := serverCmd.Process.Kill(); err != nil {
			log.Printf("停止服务器程序失败: %v", err)
		} else {
			log.Println("服务器程序已停止")
		}
	}
}

// monitorServer 监控服务器程序，如果退出则重启
func monitorServer(serverPath string, updater *Updater) {
	for {
		if serverCmd != nil {
			// 等待进程退出
			err := serverCmd.Wait()
			if err != nil {
				log.Printf("服务器程序异常退出: %v", err)
			} else {
				log.Println("服务器程序正常退出")
			}

			// 等待一段时间后重启
			log.Println("等待 3 秒后重启服务器程序...")
			time.Sleep(3 * time.Second)

			// 检查是否有新版本需要更新
			if updater.HasPendingUpdate() {
				log.Println("检测到待更新版本，等待更新完成...")
				// 等待更新完成
				for updater.HasPendingUpdate() {
					time.Sleep(1 * time.Second)
				}
			}

			// 重启服务器
			startServer(serverPath)
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}

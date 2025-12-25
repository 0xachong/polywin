//go:build !server
// +build !server

package main

import (
	"flag"
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
	checkInterval    = flag.Duration("check-interval", 5*time.Minute, "更新检查间隔时间")
	repoURL          = flag.String("repo", "https://github.com/0xachong/polywin.git", "Git 仓库 URL")
	targetExecutable = flag.String("target", "server.exe", "目标可执行文件名（需要更新的程序）")
	enableAutoUpdate = flag.Bool("auto-update", true, "是否启用自动更新")
)

var serverCmd *exec.Cmd

func main() {
	flag.Parse()

	log.Printf("PolyWin 守护程序启动，版本: %s", version)
	log.Printf("目标程序: %s", *targetExecutable)
	log.Printf("Git 仓库: %s", *repoURL)
	log.Printf("更新检查间隔: %v", *checkInterval)

	// 获取当前目录
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("获取可执行文件路径失败: %v", err)
	}
	execDir := filepath.Dir(execPath)
	targetPath := filepath.Join(execDir, *targetExecutable)

	// 检查目标程序是否存在
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		log.Printf("目标程序 %s 不存在，尝试构建...", targetPath)
		if err := buildServer(execDir); err != nil {
			log.Fatalf("构建目标程序失败: %v", err)
		}
	}

	// 创建更新器
	updater := NewUpdater(&UpdaterConfig{
		RepoURL:          *repoURL,
		CheckInterval:    *checkInterval,
		EnableAutoUpdate: *enableAutoUpdate,
		CurrentVersion:   version,
		TargetExecutable: *targetExecutable,
		TargetPath:       targetPath,
	})

	// 启动更新检查协程
	if *enableAutoUpdate {
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

// buildServer 构建服务器程序
func buildServer(targetDir string) error {
	log.Println("开始构建服务器程序...")

	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	outputPath := filepath.Join(targetDir, "server.exe")
	buildCmd := exec.Command("go", "build", "-o", outputPath, "server.go")
	buildCmd.Dir = wd
	buildCmd.Env = os.Environ()

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return err
	}

	if len(output) > 0 {
		log.Printf("构建输出: %s", string(output))
	}

	log.Printf("服务器程序构建完成: %s", outputPath)
	return nil
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

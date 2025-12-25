package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	version = "1.0.0"
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

	// 检查目标程序是否存在，不存在则从 GitHub Releases 下载
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		log.Printf("目标程序 %s 不存在，尝试从 GitHub Releases 下载...", targetPath)
		if err := downloadServerFromGitHub(execDir); err != nil {
			log.Fatalf("无法下载目标程序: %v", err)
		}
		log.Printf("目标程序下载成功: %s", targetPath)
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

// downloadServerFromGitHub 从 GitHub Releases 下载 server.exe
func downloadServerFromGitHub(targetDir string) error {
	log.Println("正在从 GitHub Releases 下载 server.exe...")

	// GitHub Releases API
	apiURL := "https://api.github.com/repos/0xachong/polywin/releases/latest"
	
	// 获取最新版本信息
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("获取 Releases 信息失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API 返回错误状态码: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("解析 Releases 信息失败: %v", err)
	}

	// 查找 server.exe
	var serverURL string
	for _, asset := range release.Assets {
		if asset.Name == "server.exe" {
			serverURL = asset.BrowserDownloadURL
			break
		}
	}

	if serverURL == "" {
		return fmt.Errorf("未找到 server.exe，请确保 GitHub Releases 中有该文件")
	}

	log.Printf("找到 server.exe，版本: %s，开始下载...", release.TagName)

	// 下载 server.exe
	downloadResp, err := http.Get(serverURL)
	if err != nil {
		return fmt.Errorf("下载 server.exe 失败: %v", err)
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", downloadResp.StatusCode)
	}

	// 保存文件
	outputPath := filepath.Join(targetDir, "server.exe")
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer outFile.Close()

	// 复制内容
	written, err := io.Copy(outFile, downloadResp.Body)
	if err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("写入文件失败: %v", err)
	}

	log.Printf("下载完成，文件大小: %d 字节", written)
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

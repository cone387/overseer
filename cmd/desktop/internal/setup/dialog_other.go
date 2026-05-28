//go:build !windows

package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ShowDialog shows a terminal-based setup prompt on non-Windows platforms.
func ShowDialog() (*Result, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Overseer Desktop - 首次配置 ===")
	fmt.Println()

	fmt.Print("服务器地址 (默认 http://localhost:9721): ")
	serverURL, _ := reader.ReadString('\n')
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		serverURL = "http://localhost:9721"
	}

	fmt.Print("注册令牌: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("setup cancelled: token is required")
	}

	hostname, _ := os.Hostname()
	defaultName := "Desktop " + hostname
	fmt.Printf("设备名称 (默认 %s): ", defaultName)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultName
	}

	return &Result{
		ServerURL: serverURL,
		Token:     token,
		Name:      name,
	}, nil
}

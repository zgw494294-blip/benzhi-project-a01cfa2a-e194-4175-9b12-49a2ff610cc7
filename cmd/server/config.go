package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	address   string
	dataDir   string
	selfcheck bool
}

func parseConfig() (config, error) {
	var cfg config
	flag.StringVar(&cfg.address, "addr", defaultAddress, "HTTP 监听地址")
	flag.StringVar(&cfg.dataDir, "data-dir", "./data", "SQLite 与底片载荷目录")
	flag.BoolVar(&cfg.selfcheck, "selfcheck", false, "执行真实 HTTP 主流程自检后退出")
	flag.Parse()
	if portText := os.Getenv("PORT"); portText != "" && cfg.address == defaultAddress {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return cfg, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
		}
		cfg.address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	if err := validateAddress(cfg.address); err != nil {
		return cfg, err
	}
	absolute, err := filepath.Abs(cfg.dataDir)
	if err != nil {
		return cfg, err
	}
	cfg.dataDir = absolute
	return cfg, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 必须采用 host:port 格式: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("监听端口无效")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("为保护检测资料，监听地址必须是回环地址")
	}
	return nil
}

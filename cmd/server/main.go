package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := parseConfig()
	if err != nil {
		slog.Error("配置无效", "error", err)
		os.Exit(2)
	}
	if cfg.selfcheck {
		if err := runSelfcheck(context.Background(), cfg.address); err != nil {
			slog.Error("selfcheck 失败", "error", err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rt, err := buildRuntime(ctx, cfg.address, cfg.dataDir)
	if err != nil {
		slog.Error("服务装配失败", "error", err)
		os.Exit(1)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- rt.server.ListenAndServe() }()
	slog.Info("射线底片判读放行台已启动", "address", "http://"+cfg.address, "dataDir", cfg.dataDir)
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "error", err)
			rt.repository.Close()
			os.Exit(1)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := rt.close(shutdownCtx); err != nil {
		slog.Error("服务关闭失败", "error", err)
		os.Exit(1)
	}
}

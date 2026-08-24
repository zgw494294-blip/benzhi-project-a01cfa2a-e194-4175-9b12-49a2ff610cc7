package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/application"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/httpapi"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/store"
	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/webui"
)

type runtime struct {
	repository *store.SQLiteRepository
	server     *http.Server
}

func buildRuntime(ctx context.Context, address, dataDir string) (*runtime, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	repository, err := store.OpenSQLite(ctx, dataDir)
	if err != nil {
		return nil, err
	}
	payloads, err := store.NewFilePayloadStore(dataDir)
	if err != nil {
		repository.Close()
		return nil, err
	}
	ids := &store.RandomIDGenerator{}
	service := application.NewService(repository, payloads, application.SystemClock{}, ids)
	handler := httpapi.New(service, webui.NewHandler())
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	return &runtime{repository: repository, server: server}, nil
}

func (r *runtime) close(ctx context.Context) error {
	serverErr := r.server.Shutdown(ctx)
	repositoryErr := r.repository.Close()
	if serverErr != nil {
		return serverErr
	}
	return repositoryErr
}

// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/service"
)

const shutdownTimeout = 5 * time.Second

type Server struct {
	cfg        *config.Config
	svc        *service.Service
	registry   *ledger.Registry
	migrations fs.FS
	appDir     string
	switchLedger func(name string) error
	logger     *slog.Logger
	http       *http.Server
}

func NewServer(
	cfg *config.Config,
	svc *service.Service,
	registry *ledger.Registry,
	migrations fs.FS,
	appDir string,
	switchLedger func(name string) error,
	logger *slog.Logger,
) *Server {
	s := &Server{
		cfg:        cfg,
		svc:        svc,
		registry:   registry,
		migrations: migrations,
		appDir:     appDir,
		switchLedger: switchLedger,
		logger:     logger,
	}
	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	s.http = &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("server listening", "addr", listener.Addr().String())
		if err := s.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		s.logger.Info("server shutting down")
		return s.http.Shutdown(shutdownCtx)
	}
}

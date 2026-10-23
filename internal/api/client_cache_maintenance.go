package api

import (
	"context"
	"log/slog"
	"time"
)

// RunCacheMaintenance prunes on startup and hourly, independently of connectivity.
func (s *Server) RunCacheMaintenance(ctx context.Context) {
	if s.client == nil {
		return
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	s.runCacheMaintenance(ctx, ticker.C)
}

func (s *Server) runCacheMaintenance(ctx context.Context, ticks <-chan time.Time) {
	for {
		if ctx.Err() != nil {
			return
		}
		attempt, cancel := context.WithTimeout(ctx, 10*time.Minute)
		_, err := s.client.PruneCache(attempt)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("Client cache maintenance failed; will retry on the next interval", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
	}
}

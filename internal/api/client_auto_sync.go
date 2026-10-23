package api

import (
	"context"
	"time"
)

func (s *Server) wakeAutoSync() {
	select {
	case s.autoSyncWake <- struct{}{}:
	default:
	}
}

// RunAutoSync owns the service-lifetime worker. Wakeups are hints, never state:
// each attempt re-reads the durable queue and serializes with manual sync.
func (s *Server) RunAutoSync(ctx context.Context) {
	if s.client == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.wakeAutoSync()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.autoSyncWake:
		}
		attempt, cancel := context.WithTimeout(ctx, 10*time.Minute)
		peer, err := s.clientPeer(attempt)
		if err == nil {
			s.autoSyncRunning.Add(1)
			_, _ = s.client.SynchronizeAutomatic(attempt, peer)
			s.autoSyncRunning.Add(-1)
		}
		cancel()
	}
}

package api

import (
	"context"
	"time"

	"github.com/jinzhao1994/kist/internal/library"
)

// RunAIMaintenance belongs to the server lifetime, independently of extraction
// pauses. Stopping it also drops all unsaved authorization transactions.
func (s *Server) RunAIMaintenance(ctx context.Context) {
	if s.library == nil {
		return
	}
	defer func() {
		s.aiDrafts.Lock()
		defer s.aiDrafts.Unlock()
		for id := range s.aiDrafts.entries {
			s.removeSignInDraftLocked(id)
		}
	}()
	state := map[int64]library.AIAccountRenewal{}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		state = s.library.RenewAIAccounts(ctx, state)
		s.aiMu.Lock()
		s.aiRenewals = state
		s.aiMu.Unlock()
	}
}

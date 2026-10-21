package api

import (
	"context"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

// Upload transport preserves each backend's durable receipt identity.
func (s *Server) uploadReceipt(ctx context.Context, key string, request *library.Request) (sessionStatus, bool, error) {
	if s.client != nil {
		var result library.ClientRequestResult
		var found bool
		var err error
		if request != nil {
			result, found, err = s.client.CheckRequest(ctx, *request)
		} else {
			result, found, err = s.client.RequestStatus(ctx, key)
		}
		return sessionStatus{LocalID: result.LocalID, Noop: result.Noop}, found, err
	}
	var result library.RequestResult
	var found bool
	var err error
	if request != nil {
		result, found, err = s.library.CheckRequest(ctx, *request)
	} else {
		result, found, err = s.library.RequestStatus(ctx, key)
	}
	return sessionStatus{OpID: result.OpID}, found, err
}

func (s *Server) checkUploadRequest(ctx context.Context, request library.Request) (sessionStatus, bool, error) {
	return s.uploadReceipt(ctx, request.ID, &request)
}

func (s *Server) uploadTarget(ctx context.Context, box domain.BoxID, folder domain.FolderID) (string, error) {
	if s.client != nil {
		return s.client.UploadTarget(ctx, box, folder)
	}
	return s.library.UploadTarget(ctx, box, folder)
}

func (s *Server) previewUpload(ctx context.Context, box domain.BoxID, folder domain.FolderID, paths []string) (library.UploadConflicts, error) {
	if s.client != nil {
		return s.client.PreviewUpload(ctx, box, folder, paths)
	}
	return s.library.PreviewUpload(ctx, box, folder, paths)
}

func (s *Server) prepareUpload(ctx context.Context, sources []library.ImportSource, opt library.ImportOptions) (func(context.Context) error, error) {
	if s.client != nil {
		prepared, err := s.client.PrepareFiles(ctx, sources, library.ClientImportOptions{Box: opt.Box, Into: opt.Into, Hidden: opt.Hidden, ConflictMode: opt.ConflictMode})
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context) error { _, err := prepared.Commit(ctx); return err }, nil
	}
	prepared, err := s.library.PrepareFiles(ctx, sources, opt)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error { _, err := prepared.Commit(ctx); return err }, nil
}

func (s *Server) executeUpload(ctx context.Context, request library.Request, command func(context.Context) error) (sessionStatus, error) {
	if s.client != nil {
		result, err := s.client.ExecuteRequest(ctx, request, command)
		return sessionStatus{LocalID: result.LocalID, Noop: result.Noop}, err
	}
	result, err := s.library.ExecuteRequest(ctx, request, command)
	return sessionStatus{OpID: result.OpID}, err
}

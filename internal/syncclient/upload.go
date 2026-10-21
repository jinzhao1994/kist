package syncclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jinzhao1994/kist/internal/blob"
)

type uploadProgress struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Size       int64  `json:"size"`
	Offset     int64  `json:"offset"`
	ChunkBytes int64  `json:"chunk_bytes"`
}

func (u uploadProgress) valid(key string, size int64) bool {
	id, err := hex.DecodeString(u.ID)
	return err == nil && len(id) == sha256.Size && hex.EncodeToString(id) == u.ID && u.Key == key && u.Size == size && u.Offset >= 0 && u.Offset <= size && u.ChunkBytes > 0 && u.ChunkBytes <= 8<<20
}

// UploadOriginal leaves durable progress on transport failure. A subsequent
// call recreates the same session, validates its prefix against the local
// original and resumes only from the server's acknowledged offset.
func (p *Peer) UploadOriginal(ctx context.Context, name string, size int64, source io.ReadSeeker) error {
	if _, err := blob.ParseKey(name); err != nil {
		return err
	}
	if size < 0 {
		return errors.New("invalid original size")
	}
	present, err := p.HasOriginal(ctx, name)
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	var progress uploadProgress
	if err := p.json(ctx, "POST", "original-uploads", struct {
		Key  string `json:"key"`
		Size int64  `json:"size"`
	}{name, size}, &progress, 1<<20); err != nil {
		return err
	}
	if !progress.valid(name, size) {
		return errors.New("invalid original upload progress")
	}
	path := "original-uploads/" + progress.ID
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if progress.Offset > 0 {
		hash := sha256.New()
		if _, err := io.CopyN(hash, contextReader{ctx, source}, progress.Offset); err != nil {
			return err
		}
		var checked uploadProgress
		if err := p.json(ctx, "POST", path+"/prefix", struct {
			Index  int    `json:"index"`
			Offset int64  `json:"offset"`
			SHA256 string `json:"sha256"`
		}{0, progress.Offset, hex.EncodeToString(hash.Sum(nil))}, &checked, 1<<20); err != nil {
			return err
		}
		if checked != progress {
			return errors.New("original progress changed during prefix validation")
		}
	}
	buffer := make([]byte, progress.ChunkBytes)
	for progress.Offset < size {
		n := min(int64(len(buffer)), size-progress.Offset)
		data := buffer[:n]
		if _, err := io.ReadFull(contextReader{ctx, source}, data); err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		headers := http.Header{"Content-Type": {"application/octet-stream"}, "Upload-Length": {strconv.FormatInt(n, 10)}, "Upload-Sha256": {hex.EncodeToString(digest[:])}}
		response, err := p.request(ctx, "PUT", path+"/files/0?offset="+strconv.FormatInt(progress.Offset, 10), bytes.NewReader(data), headers)
		if err != nil {
			return err
		}
		var next uploadProgress
		if err := decodeResponse(response, &next, 1<<20); err != nil {
			return err
		}
		if !next.valid(name, size) || next.ID != progress.ID || next.Offset != progress.Offset+n {
			return errors.New("invalid original block acknowledgement")
		}
		progress = next
	}
	var finished uploadProgress
	if err := p.json(ctx, "POST", path+"/finish", struct{}{}, &finished, 1<<20); err != nil {
		return err
	}
	if !finished.valid(name, size) || finished.ID != progress.ID || finished.Offset != size {
		return fmt.Errorf("invalid original completion acknowledgement")
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

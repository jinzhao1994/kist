package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrContentMismatch means the content offered did not hash to the key it was
// fetched for.
var ErrContentMismatch = errors.New("content does not match the key it was fetched for")

// Restore puts content fetched from somewhere else at exactly the key it was
// fetched for, after verifying it.
//
// This is repair's primitive (spec 3.8), and it is deliberately not Put:
//
//   - Put decides where content goes by hashing it and walking the collision
//     suffixes. Restore is told where it goes. A key like "sha256:ab….1" names
//     one specific slot, and Put would happily land in slot 0.
//   - Put trusts what it is given, because its caller is an import reading the
//     user's own disk. Restore does not: it is reading from a remote, and the
//     verification is precisely why a remote is allowed to be an unattended
//     external drive or a network share.
//
// If the content does not match, nothing is written and the blob already
// there — damaged or not — is left alone. Damaged content is sometimes the
// only content there is.
func (d *Dir) Restore(ctx context.Context, r io.Reader, want Key) (Outcome, error) {
	tmp, err := os.CreateTemp(filepath.Join(d.root, "tmp"), "restore-*")
	if err != nil {
		return 0, fmt.Errorf("stage restored content: %w", err)
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		tmp.Close()
		if !keep {
			os.Remove(tmpPath)
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), readerContext{ctx: ctx, r: r}); err != nil {
		return 0, fmt.Errorf("read the content to restore: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return 0, fmt.Errorf("stage restored content: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("stage restored content: %w", err)
	}

	if got := hex.EncodeToString(h.Sum(nil)); got != want.Hex() {
		return 0, fmt.Errorf("%w: wanted %s, got %s", ErrContentMismatch, want.Hex(), got)
	}

	release, err := d.lockPlacement(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	dst := filepath.Join(d.root, want.RelPath())
	_, statErr := os.Stat(dst)
	switch {
	case errors.Is(statErr, fs.ErrNotExist):
		if err := d.commit(tmpPath, dst); err != nil {
			return 0, err
		}
		keep = true
		return OutcomeStored, nil
	case statErr != nil:
		return 0, fmt.Errorf("stat the blob being restored: %w", statErr)
	}

	// Something is already there. Either it hashes correctly, in which case
	// there was nothing to repair, or it does not and we now hold verified
	// content to put in its place.
	existing, _, err := d.rehashPath(ctx, dst)
	if err != nil {
		return 0, err
	}
	if existing == want.Hex() {
		return OutcomeDeduped, nil
	}
	if err := d.replace(tmpPath, dst); err != nil {
		return 0, err
	}
	keep = true
	d.log.Info("replaced damaged content with a verified copy", "key", want, "was", existing)
	return OutcomeRepaired, nil
}

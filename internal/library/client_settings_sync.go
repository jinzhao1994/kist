package library

import (
	"context"
)

// AdoptSyncSettings refreshes the NAS library's interpretation settings for
// future local commands. Already persisted intents retain their exact payloads.
// Callers must obtain capabilities over the authenticated sync connection.
func (c *Client) AdoptSyncSettings(ctx context.Context, caps SyncCapabilities) error {
	if err := ValidateSyncCapabilities(caps); err != nil {
		return err
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	settings, err := c.settings()
	if err != nil {
		return err
	}
	if settings.LibraryID != caps.LibraryID {
		return ErrSyncConflict
	}
	if settings.Timezone == caps.Timezone {
		return nil
	}
	return updateInstanceConfig(ctx, c.root, "", map[string]any{"client.timezone": caps.Timezone})
}

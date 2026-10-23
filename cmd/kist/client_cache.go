package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/spf13/cobra"
)

func newClientCacheCmd() *cobra.Command {
	root := &cobra.Command{Use: "cache", Short: "Inspect and manage disposable client content"}
	for _, action := range []string{"status", "prune"} {
		root.AddCommand(&cobra.Command{Use: action + " DIRECTORY", Short: map[string]string{"status": "Show cached, reserved and retained byte usage", "prune": "Evict expired or least recently used unprotected content"}[action], Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			client, err := library.OpenClient(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			defer client.Close()
			var status library.ClientCacheStatus
			if action == "status" {
				status, err = client.CacheStatus(cmd.Context())
			} else {
				status, err = client.PruneCache(cmd.Context())
			}
			if err != nil {
				return err
			}
			return printCacheStatus(cmd, status)
		}})
	}
	root.AddCommand(&cobra.Command{Use: "rebuild DIRECTORY", Short: "Recover cache metadata without changing offline selections or pending originals", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := library.OpenClient(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		defer client.Close()
		result, err := client.RebuildCache(cmd.Context())
		if err != nil {
			return err
		}
		return emit(cmd, result, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Verified: %d\nPartial: %d\nCorrupt: %d\nMissing: %d\nIn use (skipped): %d\n", result.Verified, result.Partial, result.Corrupt, result.Missing, result.Skipped)
			return err
		})
	}})
	var ageText string
	var maxBytes int64
	configure := &cobra.Command{Use: "configure DIRECTORY", Short: "Set cache retention and capacity, then prune eligible content", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("max-age") && !cmd.Flags().Changed("max-bytes") {
			return errors.New("set --max-age or --max-bytes")
		}
		client, err := library.OpenClient(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		defer client.Close()
		current, err := client.Status(cmd.Context())
		if err != nil {
			return err
		}
		if !cmd.Flags().Changed("max-age") {
			ageText = current.Settings.CacheMaxAge
		}
		age, err := config.ParseCacheMaxAge(ageText)
		if err != nil {
			return err
		}
		if !cmd.Flags().Changed("max-bytes") {
			maxBytes = current.Settings.CacheMaxBytes
		}
		status, err := client.SetCacheLimits(cmd.Context(), age, maxBytes)
		if err != nil {
			return err
		}
		return printCacheStatus(cmd, status)
	}}
	configure.Flags().StringVar(&ageText, "max-age", "", "maximum age since last access, for example 180d or 24h")
	configure.Flags().Int64Var(&maxBytes, "max-bytes", 0, "maximum cache budget in bytes, including download reservations")
	root.AddCommand(configure)
	return root
}

func printCacheStatus(cmd *cobra.Command, status library.ClientCacheStatus) error {
	return emit(cmd, status, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "Cached bytes: %d\nPartial bytes: %d\nReserved bytes: %d\nPinned bytes: %d\nCorrupt bytes: %d\nLimit bytes: %d\nOver limit: %t\n", status.CachedBytes, status.PartialBytes, status.ReservedBytes, status.PinnedBytes, status.CorruptBytes, status.LimitBytes, status.OverLimit)
		return err
	})
}

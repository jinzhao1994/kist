package main

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/syncclient"
	"github.com/spf13/cobra"
)

func newClientOfflineCmd() *cobra.Command {
	root := &cobra.Command{Use: "offline", Short: "Retain and prepare selected files or filesets for offline use"}
	var files, sets []string
	selectCmd := &cobra.Command{Use: "select DIRECTORY NAME", Short: "Replace a named selection without downloading its content", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if len(files)+len(sets) == 0 {
			return errors.New("specify --file or --fileset; use remove to cancel retention")
		}
		selection := library.ClientOfflineSelection{}
		for _, text := range files {
			id, err := domain.ParseID(text)
			if err != nil {
				return err
			}
			selection.Files = append(selection.Files, domain.FileID{ID: id})
		}
		for _, text := range sets {
			id, err := domain.ParseID(text)
			if err != nil {
				return err
			}
			selection.Filesets = append(selection.Filesets, domain.FolderID{ID: id})
		}
		client, err := library.OpenClient(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		defer client.Close()
		if err := client.SetOfflineSelection(cmd.Context(), args[1], selection); err != nil {
			return err
		}
		return emit(cmd, selection, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Retained selection %s. Run offline prepare to download missing content.\n", args[1])
			return err
		})
	}}
	selectCmd.Flags().StringArrayVar(&files, "file", nil, "file ID to retain (repeatable)")
	selectCmd.Flags().StringArrayVar(&sets, "fileset", nil, "fileset folder ID to retain, including nested members (repeatable)")
	root.AddCommand(selectCmd)
	for _, action := range []string{"list", "remove", "status", "prepare"} {
		count := 2
		use := action + " DIRECTORY NAME"
		if action == "list" {
			count = 1
			use = action + " DIRECTORY"
		}
		root.AddCommand(&cobra.Command{Use: use, Short: map[string]string{"list": "List saved offline selections", "remove": "Cancel one selection's retention without deleting files", "status": "Verify selected content available on this laptop", "prepare": "Prepare selected originals and previews for offline use"}[action], Args: cobra.ExactArgs(count), RunE: func(cmd *cobra.Command, args []string) error {
			client, err := library.OpenClient(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			defer client.Close()
			switch action {
			case "list":
				selections, err := client.OfflineSelections(cmd.Context())
				if err != nil {
					return err
				}
				return emit(cmd, struct {
					Selections map[string]library.ClientOfflineSelection `json:"selections"`
				}{selections}, func(w io.Writer) error {
					names := make([]string, 0, len(selections))
					for name := range selections {
						names = append(names, name)
					}
					slices.Sort(names)
					for _, name := range names {
						if _, err := fmt.Fprintln(w, name); err != nil {
							return err
						}
					}
					return nil
				})
			case "remove":
				if err := client.SetOfflineSelection(cmd.Context(), args[1], library.ClientOfflineSelection{}); err != nil {
					return err
				}
				return emit(cmd, struct {
					Name string `json:"name"`
				}{args[1]}, func(w io.Writer) error { _, err := fmt.Fprintf(w, "Removed retention for %s.\n", args[1]); return err })
			default:
				status, err := client.OfflineStatus(cmd.Context(), args[1])
				if err != nil {
					return err
				}
				if action == "prepare" && !status.Ready {
					status, err = client.PrepareOffline(cmd.Context(), args[1], syncclient.NewClientSource(client), runClientRenderer)
					if err != nil {
						return err
					}
				}
				return emit(cmd, status, func(w io.Writer) error {
					_, err := fmt.Fprintf(w, "Ready: %t\nAvailable originals: %d / %d\nMissing originals: %d\nAvailable previews: %d / %d\n", status.Ready, status.Available, len(status.Originals), len(status.Missing), status.PreviewsAvailable, len(status.Previews))
					return err
				})
			}
		}})
	}
	return root
}

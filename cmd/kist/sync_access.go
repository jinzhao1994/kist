package main

import (
	"fmt"
	"io"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/spf13/cobra"
)

func newSyncAccessCmd() *cobra.Command {
	root := &cobra.Command{Use: "sync-access", Short: "Manage NAS read/write synchronization credentials"}
	root.AddCommand(&cobra.Command{Use: "create", Short: "Create a credential and print its secret once", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()
			credential, err := l.AuthorizeSync(cmd.Context())
			if err != nil {
				return err
			}
			return emit(cmd, credential, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "Credential: %s\nToken (shown once): %s\n", credential.ID, credential.Token)
				return err
			})
		}})
	root.AddCommand(&cobra.Command{Use: "list", Short: "List credential IDs without revealing secrets", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()
			credentials, err := l.SyncCredentials(cmd.Context())
			if err != nil {
				return err
			}
			return emit(cmd, struct {
				Credentials []library.SyncCredential `json:"credentials"`
			}{credentials}, func(w io.Writer) error {
				for _, credential := range credentials {
					if _, err := fmt.Fprintf(w, "%s\t%s\n", credential.ID, credential.CreatedAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
						return err
					}
				}
				return nil
			})
		}})
	root.AddCommand(&cobra.Command{Use: "revoke ID", Short: "Revoke a credential", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()
			revoked, err := l.RevokeSync(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(cmd, struct {
				Revoked bool `json:"revoked"`
			}{revoked}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "Revoked: %t\n", revoked)
				return err
			})
		}})
	return root
}

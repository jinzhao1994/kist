package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/syncclient"
	"github.com/spf13/cobra"
)

func newClientCmd() *cobra.Command {
	root := &cobra.Command{Use: "client", Short: "Manage a laptop offline client"}
	root.AddCommand(newClientConnectCmd(true), newClientConnectCmd(false))
	root.AddCommand(newClientCacheCmd())
	root.AddCommand(&cobra.Command{Use: "status DIRECTORY", Short: "Show local catalog and pending synchronization state", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := library.OpenClient(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		defer client.Close()
		status, err := client.Status(cmd.Context())
		if err != nil {
			return err
		}
		return printClientStatus(cmd, status)
	}})
	root.AddCommand(&cobra.Command{Use: "sync DIRECTORY", Short: "Merge pending changes with the configured NAS", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := library.OpenClient(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		defer client.Close()
		connection, token, err := client.Connection(cmd.Context())
		if err != nil {
			return err
		}
		peer, err := syncclient.New(connection.Server, token, syncclient.Options{AllowHTTP: connection.AllowHTTP})
		if err != nil {
			return err
		}
		result, syncErr := client.Synchronize(cmd.Context(), peer)
		if syncErr != nil {
			return syncErr
		}
		if err := emit(cmd, result, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Accepted: %d\nRemaining: %d\nComplete: %t\n", result.Accepted, result.Remaining, result.Complete)
			if err == nil && result.BlockedID != "" {
				_, err = fmt.Fprintf(w, "Stopped at: %s\n", result.BlockedID)
			}
			return err
		}); err != nil {
			return err
		}
		return nil
	}})
	return root
}

func newClientConnectCmd(create bool) *cobra.Command {
	name, summary := "connect", "Replace the connection credential for the same NAS library"
	if create {
		name, summary = "init", "Initialize an offline client after authenticating the NAS"
	}
	var server string
	var tokenStdin, allowHTTP bool
	cmd := &cobra.Command{Use: name + " DIRECTORY", Short: summary, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !tokenStdin {
			return errors.New("provide the NAS credential on standard input with --token-stdin")
		}
		raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 2050))
		if err != nil {
			return errors.New("could not read the NAS credential")
		}
		if len(raw) > 2049 {
			return errors.New("NAS credential is too long")
		}
		token := strings.TrimSpace(string(raw))
		connection := library.ClientConnection{Server: server, AllowHTTP: allowHTTP}
		peer, err := syncclient.New(server, token, syncclient.Options{AllowHTTP: allowHTTP})
		if err != nil {
			return err
		}
		caps, err := peer.Capabilities(cmd.Context())
		if err != nil {
			return err
		}
		if err := library.ValidateSyncCapabilities(caps); err != nil {
			return err
		}
		var client *library.Client
		if create {
			client, err = library.CreateClient(cmd.Context(), args[0], library.ClientSettings{LibraryID: caps.LibraryID})
		} else {
			client, err = library.OpenClient(cmd.Context(), args[0])
		}
		if err != nil {
			return err
		}
		defer client.Close()
		if err := client.SetConnection(cmd.Context(), caps.LibraryID, connection, token); err != nil {
			return err
		}
		status, err := client.Status(cmd.Context())
		if err != nil {
			return err
		}
		return printClientStatus(cmd, status)
	}}
	cmd.Flags().StringVar(&server, "server", "", "NAS server URL")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read the sync credential from standard input")
	cmd.Flags().BoolVar(&allowHTTP, "allow-http", false, "explicitly permit cleartext credentials outside loopback")
	_ = cmd.MarkFlagRequired("server")
	return cmd
}

func printClientStatus(cmd *cobra.Command, status library.ClientStatus) error {
	return emit(cmd, status, func(w io.Writer) error {
		server := "not configured"
		if status.Connection != nil {
			server = status.Connection.Server
		}
		_, err := fmt.Fprintf(w, "Client: %s\nNAS library: %s\nServer: %s\nCatalog complete: %t\nConfirmed operation: %d\nPending intents: %d\n", status.Settings.ID, status.Settings.LibraryID, server, status.Catalog.Complete, status.Catalog.Confirmed.OpID, len(status.Pending))
		if err != nil {
			return err
		}
		for _, pending := range status.Pending {
			state := "queued"
			if pending.AwaitingReceipt {
				state = "awaiting receipt"
			} else if pending.Conflict != "" {
				state = "conflict"
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", pending.ID, pending.Action, state); err != nil {
				return err
			}
		}
		return nil
	})
}

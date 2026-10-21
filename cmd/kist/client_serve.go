package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/web"
	"github.com/spf13/cobra"
)

func newClientServeCmd() *cobra.Command {
	var address string
	var origins []string
	cmd := &cobra.Command{Use: "serve DIRECTORY", Short: "Serve the laptop client locally without contacting the NAS", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		opt, err := serveOptions(address, origins)
		if err != nil {
			return err
		}
		client, err := library.OpenClient(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		defer client.Close()
		opt.UI = web.Handler()
		handler, err := api.NewClient(client, opt)
		if err != nil {
			return err
		}
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		fmt.Fprintf(cmd.ErrOrStderr(), "Serving kist offline client on %s\n", listener.Addr())
		return serveHTTP(ctx, listener, handler)
	}}
	cmd.Flags().StringVar(&address, "listen", "127.0.0.1:8474", "listen address; non-loopback requires --allow-origin")
	cmd.Flags().StringSliceVar(&origins, "allow-origin", nil, "trusted origin including scheme and port; repeat for multiple origins")
	return cmd
}

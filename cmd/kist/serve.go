package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/web"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	maps := api.DefaultMapProvider()
	var address string
	var origins []string
	var maxUploadBytes int64
	var maxUploadFiles int
	cmd := &cobra.Command{Use: "serve", Short: "Serve the web app and HTTP API (local access by default)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		l, err := openLibrary(cmd)
		if err != nil {
			return err
		}
		defer l.Close()
		stored, err := l.ServiceSettings()
		if err != nil {
			return err
		}
		effective, overrides, err := resolveServiceSettings(cmd, stored)
		if err != nil {
			return err
		}
		opt, err := serveOptions(effective.Listen, effective.AllowedOrigins)
		if err != nil {
			return err
		}
		applyServiceSettings(&opt, effective, overrides)
		opt.ResolveSettings = reloadResolver(cmd, effective.Listen)
		opt.UI = web.Handler(l.PrimaryColor)
		handler, err := api.New(l, opt)
		if err != nil {
			return err
		}
		listener, err := net.Listen("tcp", effective.Listen)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		stopReload := watchConfigReload(ctx, handler, cmd.ErrOrStderr())
		defer stopReload()
		mediaCtx, cancelMedia := context.WithCancel(ctx)
		mediaDone := make(chan struct{})
		go func() { handler.RunMedia(mediaCtx); close(mediaDone) }()
		defer func() { cancelMedia(); <-mediaDone }()
		uploadDone := make(chan struct{})
		go func() { handler.RunUploads(mediaCtx); close(uploadDone) }()
		defer func() { cancelMedia(); <-uploadDone }()
		fmt.Fprintf(cmd.ErrOrStderr(), "Serving kist on %s\n", listener.Addr())
		if len(effective.AllowedOrigins) > 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "No authentication: allow access only from trusted networks or an authenticated proxy.")
		}
		return serveHTTP(ctx, listener, handler)
	}}
	cmd.Flags().StringVar(&maps.StyleURL, "map-style", maps.StyleURL, "MapLibre style URL for GPX previews")
	cmd.Flags().StringVar(&maps.TerrainTiles, "map-terrain-tiles", maps.TerrainTiles, "Terrarium elevation tile URL template; empty disables terrain")
	cmd.Flags().StringVar(&maps.TerrainAttribution, "map-terrain-attribution", maps.TerrainAttribution, "attribution HTML for the configured elevation provider")
	cmd.Flags().StringSliceVar(&maps.ResourceOrigins, "map-resource-origin", nil, "additional map resource origins (tiles, glyphs, sprites); repeat as needed")
	cmd.Flags().StringVar(&address, "listen", "127.0.0.1:8473", "listen address; non-loopback requires --allow-origin")
	cmd.Flags().StringSliceVar(&origins, "allow-origin", nil, "trusted origin including scheme and port; repeat for multiple origins")
	cmd.Flags().Int64Var(&maxUploadBytes, "max-upload-bytes", api.DefaultMaxUploadBytes, "maximum upload batch bytes (including framing for multipart)")
	cmd.Flags().IntVar(&maxUploadFiles, "max-upload-files", api.DefaultMaxUploadFiles, "maximum files per upload batch")
	return cmd
}

func serveOptions(address string, origins []string) (api.Options, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return api.Options{}, err
	}
	ip := net.ParseIP(host)
	local := host == "localhost" || (ip != nil && ip.IsLoopback())
	if !local && len(origins) == 0 {
		return api.Options{}, fmt.Errorf("non-loopback listening requires an explicit --allow-origin")
	}
	// A fixed port makes the Host allowlist independent of DNS resolution.
	if port == "0" || port == "" {
		return api.Options{}, fmt.Errorf("specify a fixed nonzero port")
	}
	opt := api.Options{AllowedOrigins: origins}
	if local {
		for _, h := range []string{host, "localhost", "127.0.0.1", "::1"} {
			opt.AllowedHosts = append(opt.AllowedHosts, net.JoinHostPort(h, port))
		}
	}
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return opt, fmt.Errorf("invalid origin %q", origin)
		}
		opt.AllowedHosts = append(opt.AllowedHosts, u.Host)
	}
	return opt, nil
}

func serveHTTP(ctx context.Context, listener net.Listener, handler http.Handler) error {
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdown); err != nil {
				_ = srv.Close()
			}
		case <-done:
		}
	}()
	err := srv.Serve(listener)
	close(done)
	<-stopped
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

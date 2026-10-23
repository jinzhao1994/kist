package main

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
)

func TestServiceSettingsOnlyExplicitFlagsOverrideFile(t *testing.T) {
	cmd := newServeCmd()
	stored := config.Defaults("library").Server
	stored.Listen = "127.0.0.1:19000"
	stored.MaxUploadFiles = 25
	got, overrides, err := resolveServiceSettings(cmd, stored)
	if err != nil || got.Listen != stored.Listen || got.MaxUploadFiles != 25 || len(overrides) != 0 {
		t.Fatalf("defaults overwrote file: %+v %v", got, err)
	}
	if err = cmd.Flags().Set("listen", "127.0.0.1:19001"); err != nil {
		t.Fatal(err)
	}
	got, overrides, err = resolveServiceSettings(cmd, stored)
	if err != nil || got.Listen != "127.0.0.1:19001" || got.MaxUploadFiles != 25 || len(overrides) != 1 || overrides[0] != "server.listen" {
		t.Fatalf("explicit override: %+v %v %v", got, overrides, err)
	}
	client := newClientServeCmd()
	got, _, err = resolveServiceSettings(client, config.Defaults("client").Server)
	if err != nil || got.Listen != "127.0.0.1:8474" {
		t.Fatalf("client defaults: %v", err)
	}
}

func TestReloadResolverKeepsListenerAndExplicitLimits(t *testing.T) {
	cmd := newServeCmd()
	if err := cmd.Flags().Set("max-upload-files", "19"); err != nil {
		t.Fatal(err)
	}
	stored := config.Defaults("library").Server
	stored.Listen = "127.0.0.1:19002"
	stored.MaxUploadFiles = 7
	stored.MapStyle = "https://maps.example/style.json"
	opt, err := reloadResolver(cmd, "127.0.0.1:19001")(stored)
	if err != nil {
		t.Fatal(err)
	}
	if opt.ServiceSettings.Listen != "127.0.0.1:19001" || opt.MaxUploadFiles != 19 || opt.MapProvider.StyleURL != stored.MapStyle {
		t.Fatal(opt)
	}
	for _, host := range opt.AllowedHosts {
		if host == "127.0.0.1:19002" {
			t.Fatal("allowed unbound listener", host)
		}
	}
}

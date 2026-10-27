// Package ai defines provider-independent, on-demand inference contracts.
package ai

import "context"

const MaxTextBytes = 256 << 10
const MaxImageBytes = 20 << 20

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Image struct {
	MIME string
	Data []byte
}
type TextResult struct {
	Text     string
	Provider string
	Model    string
}

// OCRResult preserves the OCR contract while sharing generated-text provenance.
type OCRResult = TextResult

// Provider does not write archive state. Only completed results may be committed.
type Provider interface {
	Models(context.Context) ([]Model, error)
	OCR(context.Context, string, Image) (OCRResult, error)
	Describe(context.Context, string, Image, string) (TextResult, error)
	ChooseUpright(context.Context, string, []Image, bool) (UprightChoice, error)
}

// Credentials are private connection state, never archive metadata or API output.
type Credentials struct {
	HostID       string `toml:"host_id,omitempty"`
	Provider     string `toml:"provider,omitempty"`
	ClientID     string `toml:"client_id,omitempty"`
	Subject      string `toml:"subject,omitempty"`
	Email        string `toml:"email,omitempty"`
	AccessToken  string `toml:"access_token,omitempty"`
	RefreshToken string `toml:"refresh_token,omitempty"`
	IDToken      string `toml:"id_token,omitempty"`
	Scope        string `toml:"scope,omitempty"`
	ExpiresAt    int64  `toml:"expires_at,omitempty"`
}

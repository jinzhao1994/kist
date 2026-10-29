package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/geocode"
	"golang.org/x/text/language"
)

type Appearance struct {
	PrimaryColor string `toml:"primary_color" json:"primary_color"`
}
type Client struct {
	ParentID      domain.ID   `toml:"parent_id,omitempty" json:"parent_id"`
	Ancestors     []domain.ID `toml:"ancestors,omitempty" json:"ancestors"`
	ID            domain.ID   `toml:"id" json:"id"`
	LibraryID     domain.ID   `toml:"library_id" json:"library_id"`
	Timezone      string      `toml:"timezone" json:"timezone"`
	CacheMaxAge   string      `toml:"cache_max_age" json:"cache_max_age"`
	CacheMaxBytes int64       `toml:"cache_max_bytes" json:"cache_max_bytes"`
	Server        string      `toml:"server,omitempty" json:"server"`
	AllowHTTP     bool        `toml:"allow_http,omitempty" json:"allow_http"`
	Token         string      `toml:"token,omitempty" json:"-"`
}
type Server struct {
	Listen                string   `toml:"listen" json:"listen"`
	AllowedOrigins        []string `toml:"allowed_origins" json:"allowed_origins"`
	MaxUploadBytes        int64    `toml:"max_upload_bytes" json:"max_upload_bytes"`
	MaxUploadFiles        int      `toml:"max_upload_files" json:"max_upload_files"`
	MapStyle              string   `toml:"map_style" json:"map_style"`
	MapTerrainTiles       string   `toml:"map_terrain_tiles" json:"map_terrain_tiles"`
	MapTerrainAttribution string   `toml:"map_terrain_attribution" json:"map_terrain_attribution"`
	MapResourceOrigins    []string `toml:"map_resource_origins" json:"map_resource_origins"`
}
type Grant struct {
	Note      string    `toml:"note,omitempty" json:"note,omitempty"`
	ID        string    `toml:"id" json:"id"`
	CreatedAt time.Time `toml:"created_at" json:"created_at"`
	Digest    string    `toml:"digest" json:"-"`
}
type Sync struct {
	Grants []Grant `toml:"grants,omitempty" json:"-"`
}

// ValidCredentialNote bounds human-readable labels without changing token semantics.
func ValidCredentialNote(note string) bool {
	return utf8.ValidString(note) && utf8.RuneCountInString(note) <= 200
}

const defaultTimezone = "Asia/Shanghai"

// DefaultGeocoding returns the defaults offered for new and reset settings.
func DefaultGeocoding() Geocoding {
	return Geocoding{Automatic: true, Endpoint: geocode.DefaultEndpoint, RequestIntervalSeconds: 2, PauseOnError: true}
}

func Defaults(mode string) *Config {
	listen := "127.0.0.1:8473"
	if mode == "client" {
		listen = "127.0.0.1:8474"
	}
	legacyGeo := DefaultGeocoding()
	// Missing automatic flags predate opt-in location lookup.
	legacyGeo.Automatic = false
	return &Config{Geocoding: legacyGeo, Version: CurrentVersion, Mode: mode,
		Appearance: Appearance{PrimaryColor: "blue"},
		Library:    Library{Timezone: defaultTimezone, Language: "zh-CN"},
		Client:     Client{CacheMaxAge: "180d", CacheMaxBytes: 10 << 30},
		Server: Server{Listen: listen, AllowedOrigins: []string{}, MaxUploadBytes: 32 << 30, MaxUploadFiles: 1000,
			MapStyle: "https://tiles.openfreemap.org/styles/liberty", MapTerrainTiles: "https://elevation-tiles-prod.s3.amazonaws.com/terrarium/{z}/{x}/{y}.png", MapTerrainAttribution: `<a href="https://www.mapzen.com/rights/">Mapzen terrain</a>`, MapResourceOrigins: []string{}},
	}
}

func (c *Config) Validate() error {
	if c.RuntimeStorage < 0 || c.RuntimeStorage > 1 {
		return fmt.Errorf("unsupported runtime storage version")
	}
	if len(c.Library.Language) > 64 || c.Library.Language == "" {
		return fmt.Errorf("library language must be a BCP 47 language tag")
	}
	if _, err := language.Parse(c.Library.Language); err != nil {
		return fmt.Errorf("invalid library language")
	}

	if c.Geocoding.RequestIntervalSeconds < 0 || c.Geocoding.RequestIntervalSeconds > 86400 {
		return fmt.Errorf("location request interval must be between 0 and 86400 seconds")
	}
	if c.Geocoding.AfterOp < 0 || len(c.Geocoding.Endpoint) > 2048 {
		return fmt.Errorf("invalid geocoding settings")
	}
	if err := geocode.ValidateEndpoint(c.Geocoding.Endpoint); err != nil {
		return err
	}
	if len(c.AIOptions.Model) > 200 || len(c.AIOptions.ProxyURL) > 2048 || c.AIOptions.AfterOp < 0 || ((c.AIOptions.AutoFacts || c.AIOptions.AutoOCR || c.AIOptions.AutoDesc || c.AIOptions.AutoOrientation) && c.AIOptions.Model == "") || c.AIOptions.FactsAfterOp < 0 || c.AIOptions.DescAfterOp < 0 || c.AIOptions.OrientationAfterOp < 0 {
		return fmt.Errorf("invalid AI options")
	}
	if _, err := ai.HTTPClient(c.AIOptions.ProxyURL, time.Second); err != nil {
		return err
	}

	if c.Mode != "library" && c.Mode != "client" {
		return fmt.Errorf("unknown instance mode")
	}
	if !slices.Contains([]string{"gray", "green", "yellow", "pink", "blue", "orange"}, c.Appearance.PrimaryColor) {
		return fmt.Errorf("invalid primary color")
	}
	if c.Version >= 2 && c.Library.Timezone == "" {
		return fmt.Errorf("library timezone cannot be empty; omit it to use %s", defaultTimezone)
	}
	if _, err := c.Library.Zone(); err != nil {
		return err
	}
	if c.Mode == "client" {
		seenNodes := map[domain.ID]bool{c.Client.ID: true}
		if len(c.Client.Ancestors) > 31 {
			return fmt.Errorf("too many relay ancestors")
		}
		for _, id := range c.Client.Ancestors {
			if id.IsZero() || seenNodes[id] {
				return fmt.Errorf("cyclic relay ancestry")
			}
			seenNodes[id] = true
		}
		if len(c.Client.Ancestors) > 0 && c.Client.ParentID != c.Client.Ancestors[0] {
			return fmt.Errorf("invalid parent identity")
		}
		if c.Client.ID.IsZero() || c.Client.LibraryID.IsZero() || c.Client.Timezone == "" {
			return fmt.Errorf("client identity and timezone are required")
		}
		if c.Client.Server != "" || c.Client.Token != "" {
			if err := ValidateConnection(c.Client.Server, c.Client.AllowHTTP, c.Client.Token); err != nil {
				return err
			}
		}
		if _, err := time.LoadLocation(c.Client.Timezone); err != nil {
			return err
		}
	}
	age, err := ParseCacheMaxAge(c.Client.CacheMaxAge)
	if err != nil || age <= 0 || c.Client.CacheMaxBytes <= 0 {
		return fmt.Errorf("invalid cache limits")
	}
	host, port, err := net.SplitHostPort(c.Server.Listen)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || portNumber <= 0 || portNumber > 65535 {
		return fmt.Errorf("invalid listen address")
	}
	if c.Server.MaxUploadBytes <= 0 || c.Server.MaxUploadFiles <= 0 {
		return fmt.Errorf("upload limits must be positive")
	}
	if len(c.Server.AllowedOrigins) == 0 && host != "localhost" && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback()) {
		return fmt.Errorf("non-loopback listening requires allowed_origins")
	}
	for _, raw := range c.Server.AllowedOrigins {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || strings.Contains(u.Host, "*") {
			return fmt.Errorf("invalid allowed origin")
		}
	}
	if c.Server.MapStyle == "" {
		return fmt.Errorf("map style cannot be empty")
	}
	for _, raw := range append([]string{c.Server.MapStyle, c.Server.MapTerrainTiles}, c.Server.MapResourceOrigins...) {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "\r\n\t ;'\"\\") {
			return fmt.Errorf("invalid map URL")
		}
		if u.Host == "" && strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
			continue
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(u.Host, "*{}") {
			return fmt.Errorf("invalid map URL")
		}
	}
	seen := map[string]bool{}
	for _, g := range c.Sync.Grants {
		d, err := hex.DecodeString(g.Digest)
		if err != nil || len(d) != sha256.Size || g.ID == "" || seen[g.ID] || g.CreatedAt.IsZero() || !ValidCredentialNote(g.Note) {
			return fmt.Errorf("invalid sync credential")
		}
		seen[g.ID] = true
	}
	return nil
}

func Encode(value any) ([]byte, error) {
	var b bytes.Buffer
	err := toml.NewEncoder(&b).Encode(value)
	return b.Bytes(), err
}
func Revision(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

var ErrChanged = errors.New("configuration changed; reload before saving")

// Update is called under the instance configuration lock. An empty revision is
// reserved for internal read-modify-write operations that already hold it.
func Update(path, revision string, changes map[string]any) error {
	return UpdateValidated(path, revision, changes, nil)
}

// UpdateValidated validates runtime prerequisites before publishing the file.
// The caller holds the instance configuration lock.
func UpdateValidated(path, revision string, changes map[string]any, validate func(*Config) error) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if revision != "" && Revision(raw) != revision {
		return ErrChanged
	}
	if _, err = Decode(raw); err != nil {
		return err
	}
	var values map[string]any
	if _, err = toml.Decode(string(raw), &values); err != nil {
		return err
	}
	for key, value := range changes {
		parts := strings.Split(key, ".")
		node := values
		for _, part := range parts[:len(parts)-1] {
			next, ok := node[part].(map[string]any)
			if !ok {
				next = map[string]any{}
				node[part] = next
			}
			node = next
		}
		leaf := parts[len(parts)-1]
		if value == nil {
			delete(node, leaf)
		} else {
			node[leaf] = value
		}
	}
	canonical, err := Encode(values)
	if err != nil {
		return err
	}
	candidate, err := Decode(canonical)
	if err != nil {
		return err
	}
	if validate != nil {
		if err := validate(candidate); err != nil {
			return err
		}
	}
	// Preserve common hand-written files, including comments and untouched tables.
	// Complex TOML syntax falls back to semantic encoding, never string guessing.
	var normalized map[string]any
	if _, err = toml.Decode(string(canonical), &normalized); err != nil {
		return err
	}
	next := patchLines(raw, changes)
	var patched map[string]any
	if _, err = toml.Decode(string(next), &patched); err != nil || !reflect.DeepEqual(patched, normalized) {
		next = canonical
	}
	latest, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(latest, raw) {
		return ErrChanged
	}
	return WriteAtomic(path, next)
}

func patchLines(raw []byte, changes map[string]any) []byte {
	lines := strings.Split(string(raw), "\n")
	section := ""
	seen := map[string]bool{}
	out := []string{}
	sections := map[string]bool{"": true}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			if end := strings.Index(trim, "]"); end >= 0 {
				section = trim[1:end]
				sections[section] = true
			}
		}
		key, _, ok := strings.Cut(trim, "=")
		if ok && !strings.HasPrefix(trim, "#") {
			key = strings.TrimSpace(key)
			if section != "" {
				key = section + "." + key
			}
			if value, exists := changes[key]; exists {
				seen[key] = true
				if value == nil {
					continue
				}
				encoded, err := Encode(map[string]any{"v": value})
				if err != nil {
					return nil
				}
				_, v, _ := strings.Cut(string(encoded), "=")
				if strings.Contains(strings.TrimSpace(v), "\n") {
					return nil
				}
				prefix, _, _ := strings.Cut(line, "=")
				line = prefix + "=" + strings.TrimSuffix(v, "\n") + inlineComment(line)
			}
		}
		out = append(out, line)
	}
	for key, value := range changes {
		if seen[key] || value == nil {
			continue
		}
		parts := strings.Split(key, ".")
		table := strings.Join(parts[:len(parts)-1], ".")
		encoded, err := Encode(map[string]any{parts[len(parts)-1]: value})
		if err != nil || strings.Contains(strings.TrimSpace(string(encoded)), "\n") {
			return nil
		}
		line := strings.TrimSuffix(string(encoded), "\n")
		if table == "" {
			out = append([]string{line}, out...)
			continue
		}
		if !sections[table] {
			out = append(out, "", "["+table+"]", line)
			sections[table] = true
			continue
		}
		found := false
		for i, existing := range out {
			if strings.TrimSpace(existing) == "["+table+"]" {
				out = append(out[:i+1], append([]string{line}, out[i+1:]...)...)
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return []byte(strings.Join(out, "\n"))
}

func inlineComment(line string) string {
	quote := rune(0)
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == '#' {
			return " " + line[i:]
		}
	}
	return ""
}

func sparse(c *Config) ([]byte, error) {
	raw, err := Encode(c)
	if err != nil {
		return nil, err
	}
	defaults, err := Encode(Defaults(c.Mode))
	if err != nil {
		return nil, err
	}
	var values, base map[string]any
	if _, err = toml.Decode(string(raw), &values); err != nil {
		return nil, err
	}
	if _, err = toml.Decode(string(defaults), &base); err != nil {
		return nil, err
	}
	var prune func(map[string]any, map[string]any)
	prune = func(node, def map[string]any) {
		for k, v := range node {
			if child, ok := v.(map[string]any); ok {
				other, _ := def[k].(map[string]any)
				prune(child, other)
				if len(child) == 0 {
					delete(node, k)
				}
			} else if reflect.DeepEqual(v, def[k]) {
				delete(node, k)
			}
		}
	}
	prune(values, base)
	if c.Mode == "library" && c.Library.Timezone != "" {
		library, ok := values["library"].(map[string]any)
		if !ok {
			library = map[string]any{}
			values["library"] = library
		}
		library["timezone"] = c.Library.Timezone
	}
	values["version"] = c.Version
	values["mode"] = c.Mode
	return Encode(values)
}

func WriteAtomic(path string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".kist-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// ValidateConnection checks a stored endpoint and credential without echoing either.
func ValidateConnection(server string, allowHTTP bool, token string) error {
	u, err := url.Parse(server)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("sync requires an HTTP(S) server URL without credentials, query or fragment")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := strings.EqualFold(u.Hostname(), "localhost") || ip != nil && ip.IsLoopback()
	if u.Scheme == "http" && !loopback && !allowHTTP {
		return errors.New("sync requires HTTPS outside loopback unless cleartext transport is explicitly enabled")
	}
	if token == "" || len(token) > 2048 || strings.IndexFunc(token, func(r rune) bool { return r <= 0x20 || r >= 0x7f }) >= 0 {
		return errors.New("a valid sync credential is required")
	}
	return nil
}

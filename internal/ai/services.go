package ai

// Services is instance-private configuration, never archive data or API output.
// Numbers identify references; IDs are independently editable display names.
type Services struct {
	NextProxy   int64              `toml:"next_proxy"`
	NextAccount int64              `toml:"next_account"`
	Proxies     []Proxy            `toml:"proxies"`
	Accounts    []Account          `toml:"accounts"`
	Features    map[string]Feature `toml:"features"`
}

type Proxy struct {
	Number     int64  `toml:"number"`
	ID         string `toml:"id"`
	Type       string `toml:"type"`
	URL        string `toml:"url,omitempty" json:"-"`
	Host       string `toml:"host,omitempty"`
	Port       int    `toml:"port,omitempty"`
	Password   string `toml:"password,omitempty" json:"-"`
	ServerName string `toml:"server_name,omitempty"`
}

type Account struct {
	Number      int64       `toml:"number"`
	ID          string      `toml:"id"`
	Type        string      `toml:"type"`
	Proxy       int64       `toml:"proxy"`
	APIKey      string      `toml:"api_key,omitempty" json:"-"`
	Credentials Credentials `toml:"credentials,omitempty" json:"-"`
}

// Feature resolves one independently configured extraction route. AfterOp is
// an internal import boundary, not a caller-selectable inference parameter.
type Feature struct {
	Account         int64  `toml:"account" json:"account"`
	Model           string `toml:"model" json:"model"`
	ReasoningEffort string `toml:"reasoning_effort,omitempty" json:"reasoning_effort"`
	ImageDetail     string `toml:"image_detail,omitempty" json:"image_detail"`
	Automatic       bool   `toml:"automatic" json:"automatic"`
	PauseOnError    *bool  `toml:"pause_on_error,omitempty" json:"pause_on_error,omitempty"`
	AfterOp         int64  `toml:"after_op" json:"-"`
}

// PausesOnError keeps omission on the established default, while an explicit
// false allows subsequent jobs to proceed without retrying the failed job.
func (f Feature) PausesOnError() bool { return f.PauseOnError == nil || *f.PauseOnError }

const (
	ProxyHTTP      = "http"
	ProxyTrojan    = "trojan"
	AccountChatGPT = "chatgpt"
	AccountOpenAI  = "openai"
)

// FeatureNames is the registry of configurable AI extraction routes. Local
// metadata and GPS are deliberately not AI account consumers.
func FeatureNames() []string {
	names := []string{}
	for _, definition := range FeatureDefinitions() {
		names = append(names, definition.Name)
	}
	return names
}

type FeatureDefinition struct {
	Models       []string            `json:"models,omitempty"`
	DefaultModel string              `json:"default_model,omitempty"`
	Name         string              `json:"name"`
	Parameters   []string            `json:"parameters"`
	AccountTypes []string            `json:"account_types"`
	Options      map[string][]string `json:"options"`
}

func ParameterValues(name string) []string {
	switch name {
	case "reasoning_effort":
		return []string{"", "none", "minimal", "low", "medium", "high", "xhigh", "max"}
	case "image_detail":
		return []string{"", "auto", "low", "high", "original"}
	default:
		return nil
	}
}

func FeatureDefinitions() []FeatureDefinition {
	definitions := []FeatureDefinition{}
	for _, name := range []string{"orientation", "ocr", "transcription", "desc", "facts"} {
		if name == "transcription" {
			definitions = append(definitions, FeatureDefinition{Name: name, Models: TranscriptionModels(), DefaultModel: DefaultTranscriptionModel, Parameters: []string{}, AccountTypes: []string{AccountOpenAI}, Options: map[string][]string{}})
			continue
		}
		definitions = append(definitions, FeatureDefinition{Name: name, Parameters: []string{"reasoning_effort", "image_detail"}, AccountTypes: []string{AccountChatGPT, AccountOpenAI}, Options: map[string][]string{"reasoning_effort": ParameterValues("reasoning_effort"), "image_detail": ParameterValues("image_detail")}})
	}
	return definitions
}

func (s Services) Account(number int64) (Account, bool) {
	for _, account := range s.Accounts {
		if account.Number == number {
			return account, true
		}
	}
	return Account{}, false
}

func (s Services) Proxy(number int64) (Proxy, bool) {
	for _, proxy := range s.Proxies {
		if proxy.Number == number {
			return proxy, true
		}
	}
	return Proxy{}, false
}

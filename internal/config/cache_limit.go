package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// CacheBytes stores an optional byte ceiling. Zero means explicitly unlimited;
// TOML encodes that as "inf", never as an ambiguous zero-byte quota.
type CacheBytes int64

func (b *CacheBytes) UnmarshalTOML(value any) error {
	switch v := value.(type) {
	case int64:
		if v > 0 {
			*b = CacheBytes(v)
			return nil
		}
	case string:
		if strings.EqualFold(strings.TrimSpace(v), "inf") {
			*b = 0
			return nil
		}
		n, err := humanize.ParseBytes(strings.TrimSpace(v))
		if err == nil && n > 0 && n <= 9007199254740991 {
			*b = CacheBytes(n)
			return nil
		}
	}
	return fmt.Errorf("cache capacity requires a positive byte size or inf")
}
func (b CacheBytes) MarshalTOML() ([]byte, error) {
	if b == 0 {
		return []byte(`"inf"`), nil
	}
	if b < 0 {
		return nil, fmt.Errorf("invalid cache capacity")
	}
	return []byte(strconv.FormatInt(int64(b), 10)), nil
}

// ParseCacheRetention extends the shared duration syntax with an explicit
// unlimited value. Existing client/video settings retain their finite policy.
func ParseCacheRetention(value string) (age time.Duration, err error) {
	if strings.EqualFold(strings.TrimSpace(value), "inf") {
		return 0, nil
	}
	duration, err := ParseCacheMaxAge(value)
	return duration, err
}

package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ParseCacheMaxAge accepts Go durations or an integer number of 24-hour days.
func ParseCacheMaxAge(value string) (time.Duration, error) {
	if strings.HasSuffix(value, "d") {
		days, err := strconv.ParseInt(strings.TrimSuffix(value, "d"), 10, 64)
		if err != nil || days <= 0 || days > math.MaxInt64/int64(24*time.Hour) {
			return 0, fmt.Errorf("invalid cache retention time %q", value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	age, err := time.ParseDuration(value)
	if err != nil || age <= 0 {
		return 0, fmt.Errorf("invalid cache retention time %q", value)
	}
	return age, nil
}

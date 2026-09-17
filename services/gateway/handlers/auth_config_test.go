package api

import "testing"

func TestLoginRateLimitSwitch(t *testing.T) {
	for _, tc := range []struct {
		value    string
		disabled bool
	}{{"", false}, {"true", false}, {"invalid", false}, {"false", true}, {" FALSE ", true}} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("LOGIN_RATE_LIMIT_ENABLED", tc.value)
			if got := loginProtectionConfig().DisableRateLimit; got != tc.disabled {
				t.Fatalf("DisableRateLimit = %v, want %v", got, tc.disabled)
			}
		})
	}
}

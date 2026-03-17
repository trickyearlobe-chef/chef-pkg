package cmd

import (
	"testing"
)

func TestMaskSecret_LongValue(t *testing.T) {
	// Values >= 10 chars should show first 4 + **** + last 4
	got := maskSecret("abcdefghijklmnop")
	want := "abcd****mnop"
	if got != want {
		t.Errorf("maskSecret(abcdefghijklmnop) = %q, want %q", got, want)
	}
}

func TestMaskSecret_UUID(t *testing.T) {
	got := maskSecret("550e8400-e29b-41d4-a716-446655440000")
	want := "550e****0000"
	if got != want {
		t.Errorf("maskSecret(uuid) = %q, want %q", got, want)
	}
}

func TestMaskSecret_ShortValue(t *testing.T) {
	// Values < 10 chars should be fully masked
	got := maskSecret("secret")
	want := "****"
	if got != want {
		t.Errorf("maskSecret(secret) = %q, want %q", got, want)
	}
}

func TestMaskSecret_ExactlyTen(t *testing.T) {
	got := maskSecret("1234567890")
	want := "1234****7890"
	if got != want {
		t.Errorf("maskSecret(1234567890) = %q, want %q", got, want)
	}
}

func TestMaskSecret_Empty(t *testing.T) {
	got := maskSecret("")
	want := ""
	if got != want {
		t.Errorf("maskSecret(empty) = %q, want %q", got, want)
	}
}

func TestIsSecretKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"chef.license_id", true},
		{"nexus.password", true},
		{"artifactory.password", true},
		{"artifactory.token", true},
		{"nexus.url", false},
		{"chef.channel", false},
		{"chef.base_url", false},
		{"download.dest", false},
	}
	for _, tt := range tests {
		got := isSecretKey(tt.key)
		if got != tt.want {
			t.Errorf("isSecretKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

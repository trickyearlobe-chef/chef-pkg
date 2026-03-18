package cmd

import "testing"

func TestShouldCleanNexusRepo(t *testing.T) {
	tests := []struct {
		name string
		repo string
		want bool
	}{
		{"chef prefix", "chef-el9-x86_64-yum", true},
		{"legacy product prefix", "inspec-el9-x86_64-yum", true},
		{"chef ice prefix", "chef-ice-linux-x86_64-yum", true},
		{"other repo", "myorg-el9-x86_64-yum", false},
		{"partial prefix", "chefish-el9-x86_64-yum", false},
	}

	for _, tt := range tests {
		got := shouldCleanNexusRepo(tt.repo)
		if got != tt.want {
			t.Fatalf("%s: shouldCleanNexusRepo(%q) = %v, want %v", tt.name, tt.repo, got, tt.want)
		}
	}
}

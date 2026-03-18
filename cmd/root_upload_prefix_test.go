package cmd

import "testing"

func TestResolveRepoPrefix_DefaultsToChef(t *testing.T) {
	got := resolveRepoPrefix("")
	want := "chef"
	if got != want {
		t.Fatalf("resolveRepoPrefix(\"\") = %q, want %q", got, want)
	}
}

func TestResolveRepoPrefix_UsesCustomValue(t *testing.T) {
	got := resolveRepoPrefix("mycompany-chef")
	want := "mycompany-chef"
	if got != want {
		t.Fatalf("resolveRepoPrefix(custom) = %q, want %q", got, want)
	}
}

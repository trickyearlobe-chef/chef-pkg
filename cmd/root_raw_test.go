package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestRunRawGet_WritesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stable/chef/versions/all" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("license_id") != "test-license" {
			t.Fatalf("unexpected license_id: %s", r.URL.Query().Get("license_id"))
		}
		if r.URL.Query().Get("foo") != "bar" {
			t.Fatalf("unexpected foo: %s", r.URL.Query().Get("foo"))
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	viper.Set("chef.license_id", "test-license")
	viper.Set("chef.base_url", server.URL)
	defer viper.Reset()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringSlice("query", nil, "")
	if err := cmd.Flags().Set("query", "foo=bar"); err != nil {
		t.Fatal(err)
	}

	if err := runRawGet(cmd, []string{"stable/chef/versions/all"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.Close()
	got, _ := io.ReadAll(r)
	if !bytes.Contains(got, []byte(`{"ok":true}`)) {
		t.Fatalf("unexpected stdout: %s", string(got))
	}
}

func TestStripWrappingQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"stable/chef/versions/all"`, `stable/chef/versions/all`},
		{`'stable/chef/versions/all'`, `stable/chef/versions/all`},
		{`stable/chef/versions/all`, `stable/chef/versions/all`},
	}

	for _, tt := range tests {
		got := stripWrappingQuotes(tt.input)
		if got != tt.want {
			t.Fatalf("stripWrappingQuotes(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

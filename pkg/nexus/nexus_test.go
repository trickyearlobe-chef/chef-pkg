package nexus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoExists_True(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/rest/v1/repositories/chef-el9-x86_64-yum" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	exists, err := client.RepoExists(context.Background(), "chef-el9-x86_64-yum")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected repo to exist")
	}
}

func TestRepoExists_False(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	exists, err := client.RepoExists(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected repo to not exist")
	}
}

func TestCreateRepo_Yum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/service/rest/v1/repositories/yum/hosted" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		if payload["name"] != "chef-el9-x86_64-yum" {
			t.Errorf("unexpected name: %v", payload["name"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	err := client.CreateRepo(context.Background(), "chef-el9-x86_64-yum", "yum")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepo_Apt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/rest/v1/repositories/apt/hosted" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	err := client.CreateRepo(context.Background(), "chef-ubuntu-jammy-amd64-apt", "apt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepo_Raw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/rest/v1/repositories/raw/hosted" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	err := client.CreateRepo(context.Background(), "chef-windows2019-x86_64-raw", "raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepo_Nuget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/rest/v1/repositories/nuget/hosted" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	err := client.CreateRepo(context.Background(), "chef-windows2019-x86_64-nuget", "nuget")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid repo config"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	err := client.CreateRepo(context.Background(), "bad-repo", "yum")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpload_Success(t *testing.T) {
	var receivedPath string
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	// Create a temp file to upload
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "chef_18.4.12-1_amd64.deb")
	os.WriteFile(tmpFile, []byte("package content"), 0644)

	client := NewClient(server.URL, "admin", "password")
	err := client.Upload(context.Background(), "chef-el9-x86_64-yum", "chef/18.4.12/chef_18.4.12-1_amd64.deb", tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := "/repository/chef-el9-x86_64-yum/chef/18.4.12/chef_18.4.12-1_amd64.deb"
	if receivedPath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, receivedPath)
	}
	if receivedBody != "package content" {
		t.Errorf("expected body 'package content', got %q", receivedBody)
	}
}

func TestUpload_FileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "password")
	err := client.Upload(context.Background(), "repo", "path", "/nonexistent/file")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpload_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("access denied"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.rpm")
	os.WriteFile(tmpFile, []byte("content"), 0644)

	client := NewClient(server.URL, "admin", "password")
	err := client.Upload(context.Background(), "repo", "path/test.rpm", tmpFile)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestAuth_Header(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth")
		}
		if user != "admin" || pass != "secret" {
			t.Errorf("unexpected creds: %s/%s", user, pass)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	_, err := client.RepoExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

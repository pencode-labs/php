package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func writeFile(t *testing.T, name, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func zipData(t *testing.T, name, data string, mode os.FileMode) []byte {
	t.Helper()
	var buffer bytes.Buffer
	w := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	file, err := w.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func setupNode(t *testing.T) {
	t.Helper()
	t.Setenv("PENCODE_PHP_XDEBUG_PATH", "")
	bin := t.TempDir()
	node := "node"
	if runtime.GOOS == "windows" {
		node = "node.exe"
	}
	writeFile(t, filepath.Join(bin, node), "runtime", 0o755)
	t.Setenv("PATH", bin)
}

func TestXdebugInstallAndOfflineCache(t *testing.T) {
	setupNode(t)
	asset := "php-debug-1.2.3.vsix"
	payload := zipData(t, "extension/out/phpDebug.js", "console.log('adapter')", 0o644)
	downloads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.2.3", "assets": []map[string]string{{"name": asset, "browser_download_url": "https://example.test/asset"}}})
			return
		}
		downloads++
		w.Write(payload)
	}))
	defer server.Close()
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(server.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})}
	tools := &toolManager{directory: filepath.Join(t.TempDir(), "cache with spaces"), client: client, log: io.Discard}
	command, err := tools.ensureXdebug(t.Context())
	if err != nil || command.path == "" {
		t.Fatal(command, err)
	}
	if len(command.args) != 1 {
		t.Fatal("adapter must use stdio without --server", command)
	}
	server.Close()
	if _, err := tools.ensureXdebug(t.Context()); err != nil || downloads != 1 {
		t.Fatal("cached launch accessed network", downloads, err)
	}
	entries, err := os.ReadDir(tools.directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "xdebug" {
		t.Fatal("staging directory leaked", entries, err)
	}
}

func TestXdebugScriptOverride(t *testing.T) {
	setupNode(t)
	tools := &toolManager{directory: filepath.Join(t.TempDir(), "absent"), log: io.Discard}
	script := filepath.Join(t.TempDir(), "phpDebug.js")
	t.Setenv("PENCODE_PHP_XDEBUG_PATH", script)
	if _, err := tools.ensureXdebug(t.Context()); err == nil {
		t.Fatal("accepted missing adapter")
	}
	writeFile(t, script, "adapter", 0o644)
	command, err := tools.ensureXdebug(t.Context())
	if err != nil || len(command.args) != 1 || command.args[0] != script {
		t.Fatal(command, err)
	}
	if _, err := os.Stat(tools.directory); !os.IsNotExist(err) {
		t.Fatal("adapter override touched cache", err)
	}
}

func TestFailedDownloadDoesNotPublishCache(t *testing.T) {
	setupNode(t)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Status: "503 Service Unavailable", Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})}
	tools := &toolManager{directory: t.TempDir(), client: client, log: io.Discard}
	if _, err := tools.ensureXdebug(t.Context()); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatal("download error missing", err)
	}
	entries, err := os.ReadDir(tools.directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("failed download left files", entries, err)
	}
}

func TestArchiveRejectsTraversalAndLinks(t *testing.T) {
	for _, entry := range []struct {
		name string
		mode os.FileMode
	}{{"../outside", 0o644}, {"extension/../../outside", 0o644}, {"/absolute", 0o644}, {`extension\outside`, 0o644}, {"extension/link", os.ModeSymlink | 0o777}} {
		t.Run(entry.name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "test.zip")
			if err := os.WriteFile(archive, zipData(t, entry.name, "bad", entry.mode), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := unpack(archive, t.TempDir()); err == nil {
				t.Fatal("accepted unsafe entry")
			}
		})
	}
}

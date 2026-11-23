package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type toolCommand struct {
	path string
	args []string
}

type toolManager struct {
	log       io.Writer
	client    *http.Client
	directory string
}

func newTools() (*toolManager, error) {
	directory := os.Getenv("PENCODE_PHP_TOOLS_DIR")
	if directory == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		directory = filepath.Join(cache, "pencode", "php")
	}
	directory, err := filepath.Abs(directory)
	return &toolManager{directory: directory, client: &http.Client{Timeout: 5 * time.Minute}, log: os.Stderr}, err
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func (t *toolManager) ensureXdebug(ctx context.Context) (toolCommand, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return toolCommand{}, fmt.Errorf("install Node.js on the workspace machine to run Xdebug: %w", err)
	}
	if script := os.Getenv("PENCODE_PHP_XDEBUG_PATH"); script != "" {
		if !regularFile(script) {
			return toolCommand{}, fmt.Errorf("PENCODE_PHP_XDEBUG_PATH must point to an existing phpDebug.js")
		}
		return toolCommand{node, []string{script}}, nil
	}
	destination := filepath.Join(t.directory, "xdebug")
	adapter := filepath.Join("extension", "out", "phpDebug.js")
	command := toolCommand{node, []string{filepath.Join(destination, adapter)}}
	if regularFile(command.args[0]) {
		return command, nil
	}
	if err := os.MkdirAll(t.directory, 0o755); err != nil {
		return toolCommand{}, err
	}
	stage, err := os.MkdirTemp(t.directory, ".install-xdebug-")
	if err != nil {
		return toolCommand{}, err
	}
	defer os.RemoveAll(stage)
	fmt.Fprintln(t.log, "PHP: installing Xdebug adapter…")
	if err := t.installXdebug(ctx, stage); err != nil {
		return toolCommand{}, fmt.Errorf("install Xdebug adapter: %w", err)
	}
	if !regularFile(filepath.Join(stage, adapter)) {
		return toolCommand{}, fmt.Errorf("installation did not produce phpDebug.js")
	}
	// A concurrent launch may already have published a complete installation.
	if err := os.Rename(stage, destination); err != nil && !regularFile(command.args[0]) {
		return toolCommand{}, err
	}
	fmt.Fprintf(t.log, "PHP: Xdebug adapter ready at %s\n", destination)
	return command, nil
}

func (t *toolManager) installXdebug(ctx context.Context, stage string) error {
	url, err := t.releaseAsset(ctx)
	if err != nil {
		return err
	}
	archive := filepath.Join(stage, "download")
	if err := t.download(ctx, url, archive); err != nil {
		return err
	}
	if err := unpack(archive, stage); err != nil {
		return err
	}
	return os.Remove(archive)
}

func (t *toolManager) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pencode-php")
	response, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, response.Status)
	}
	return response, nil
}

func (t *toolManager) releaseAsset(ctx context.Context) (string, error) {
	response, err := t.get(ctx, "https://api.github.com/repos/xdebug/vscode-php-debug/releases/latest")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var release struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&release); err != nil {
		return "", err
	}
	asset := "php-debug-" + strings.TrimPrefix(release.Tag, "v") + ".vsix"
	for _, entry := range release.Assets {
		if entry.Name == asset && entry.URL != "" {
			return entry.URL, nil
		}
	}
	return "", fmt.Errorf("Xdebug release has no asset %q", asset)
}

func (t *toolManager) download(ctx context.Context, url, destination string) error {
	response, err := t.get(ctx, url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	n, err := io.Copy(file, io.LimitReader(response.Body, (128<<20)+1))
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if n == 0 || n > 128<<20 {
		return fmt.Errorf("empty or oversized download")
	}
	return closeErr
}

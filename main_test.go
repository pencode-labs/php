package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pencode-labs/pencode/debugger"
	"github.com/pencode-labs/pencode/peb"
)

func TestRegistrationContributesOnlyXdebug(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "not-created")
	t.Setenv("PENCODE_PHP_TOOLS_DIR", directory)
	contributions := phpContributions("php.exe")
	var input, output bytes.Buffer
	writer := peb.Writer{Target: &input}
	hello, _ := peb.EncodeHello(nil, peb.Hello{Host: "test", Capabilities: peb.Capabilities})
	if err := writer.Write(peb.Initialize, 1, hello); err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(peb.Shutdown, 0, nil); err != nil {
		t.Fatal(err)
	}
	if err := peb.ServeExtension(&input, &output, contributions, peb.Handlers{}); err != nil {
		t.Fatal(err)
	}
	reader := peb.Reader{Source: &output}
	frame, err := reader.Read()
	if err != nil || frame.Kind != peb.Register || frame.ID != 1 {
		t.Fatal(frame, err)
	}
	decoded, err := peb.DecodeContributions(frame.Payload)
	if err != nil || len(decoded.LanguageServers) != 0 || len(decoded.DebugAdapters) != 1 || decoded.ActionService {
		t.Fatal(decoded, err)
	}
	adapter := decoded.DebugAdapters[0]
	templates, err := debugger.ParseConfigs([]byte(adapter.Templates), "${workspaceFolder}")
	if err != nil || len(templates) != 2 || templates[0].Adapter != "Xdebug" {
		t.Fatal("Xdebug template", templates, err)
	}
	if adapter.ID != "Xdebug" || adapter.Command != "bin/php.exe" || strings.Join(adapter.Args, " ") != "--tool xdebug" {
		t.Fatal(adapter)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("registration touched the tool cache", err)
	}
}

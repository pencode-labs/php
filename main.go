// Adapted from zed-extensions/php (Apache-2.0) for Pencode's PEB protocol.
// Tool installation runs only in launchers, never during registration.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pencode-labs/pencode/peb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) >= 3 && os.Args[1] == "--tool" {
		if os.Args[2] != "xdebug" {
			return fmt.Errorf("unknown PHP tool %q", os.Args[2])
		}
		if err := ownProcesses(); err != nil {
			return err
		}
		tools, err := newTools()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		command, err := tools.ensureXdebug(ctx)
		cancel()
		if err != nil {
			return err
		}
		return launch(command.path, append(command.args, os.Args[3:]...))
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return peb.ServeExtension(os.Stdin, os.Stdout, phpContributions(filepath.Base(self)), peb.Handlers{})
}

func phpContributions(executable string) peb.Contributions {
	command := "bin/" + executable
	return peb.Contributions{
		DebugAdapters: []peb.DebugAdapter{{ID: "Xdebug", Label: "PHP (Xdebug)", Command: command, Args: []string{"--tool", "xdebug"}, Templates: `[{"label":"PHP: 当前文件","adapter":"Xdebug","request":"launch","configuration":{"program":"${file}","cwd":"${fileDirname}","hostname":"127.0.0.1","port":0,"stopOnEntry":true,"runtimeArgs":["-dxdebug.start_with_request=yes"],"env":{"XDEBUG_MODE":"debug","XDEBUG_CONFIG":"client_host=127.0.0.1 client_port=${port}"}}},{"label":"PHP: 监听 Xdebug","adapter":"Xdebug","request":"launch","configuration":{"port":9003,"cwd":"${workspaceFolder}"}}]`}},
	}
}

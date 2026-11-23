# PHP

The extension integrates Xdebug with Pencode’s debugger. Configure PHP and Blade
language services in Pencode under **设置 → 语言服务器 → PHP**.

## Xdebug

The extension contributes the `Xdebug` stdio DAP adapter. Node.js is required on
the workspace machine. On first launch it downloads the latest
`xdebug/vscode-php-debug` VSIX from GitHub, extracts the adapter and its bundled
dependencies, and runs `extension/out/phpDebug.js` through Node.js. Xdebug itself
must be installed and enabled in the PHP runtime separately.

The adapter is cached in `<UserCacheDir>/pencode/php/xdebug` on the workspace
machine. Set `PENCODE_PHP_TOOLS_DIR` to change the cache root, or set
`PENCODE_PHP_XDEBUG_PATH` to the path of an existing `extension/out/phpDebug.js`.

To run a PHP CLI file, open it and choose **添加启动器 → PHP: 当前文件** in Debug.
The generated JSON uses `${file}` and `${fileDirname}`, resolved from the selected
editor file each time debugging starts. The adapter launches PHP with debugging
enabled and a free loopback port, and stops at the first executable line. Save
configuration changes, return to the PHP file and press F5. PHP must load the
Xdebug runtime extension (`php --ri xdebug`); downloading the Node.js adapter does
not install this PHP module. If needed, add `-dzend_extension=/path/to/xdebug.so`
to `runtimeArgs` or enable Xdebug in the PHP runtime's INI configuration.

For browser or externally launched requests, choose
**添加启动器 → PHP: 监听 Xdebug** to write the listener template
to `.pencode/debug.json` and open it for editing. Save changes to refresh the
configuration, then start it before running PHP with Xdebug enabled:

```json
[
  {
    "label": "PHP: listen for Xdebug",
    "adapter": "Xdebug",
    "request": "launch",
    "configuration": {
      "port": 9003,
      "cwd": "${workspaceFolder}"
    }
  }
]
```

Set `pathMappings` in `configuration` when PHP runs in a container or another
machine. The adapter and managed tools always run on the workspace server,
including SSH workspaces. Adapter installation logs use the Debugger Output
channel. An explicit debugger `command`/`args` overrides the contributed launcher.

## Build and install

Requires Go 1.27.0 or newer and the current Pencode peb v20 SDK.

```sh
git clone git@github.com:pencode-labs/php.git
cd php
go build -o bin/php .
go test -race ./...
```

Choose **Install from local directory** in Pencode and select this directory.
On Windows build `bin/php.exe` and change the manifest entry accordingly. The
[Pencode repository](https://github.com/pencode-labs/pencode)'s
`make install-extensions` builds and installs this extension along
with the others, adjusting executable suffixes automatically.

## Porting notes and license

The Xdebug integration is adapted from
[zed-extensions/php](https://github.com/zed-extensions/php), upstream commit
`6768398bea34ea3b2ad590ac8081eb6c1ce6bf29` (version 0.5.4).

Apache-2.0; see [LICENSE](LICENSE). Upstream copyright 2022–2025 Zed Industries,
Inc. The Go implementation and Pencode integration are modifications made in
2026 by Pencode Labs.

## Releases

Push a version tag such as `v0.1.0` to test, cross-compile and publish a GitHub
Release automatically. Linux, macOS (`darwin`) and Windows each support `amd64`
and `arm64`. Prerelease tags such as `v0.2.0-rc.1` create prereleases.

Assets use `<extension-id>-<tag>-<os>-<arch>.zip`, accompanied by `SHA256SUMS`.
Each ZIP has one top-level `<extension-id>/` directory containing `bin/`,
`resources/extension.json`, the README, license and all resource notices. The
packaged manifest uses the tag version and Windows entries include `.exe`.

Download the archive matching the **workspace server's** OS and architecture.
Extract it directly into `~/.pencode/extensions/` (Windows:
`%USERPROFILE%/.pencode/extensions/`), preserving executable permissions. For
example, `unzip php-v0.1.0-linux-amd64.zip -d ~/.pencode/extensions/` installs
`~/.pencode/extensions/php/resources/extension.json`. Restart Pencode to discover the extension.
For remote workspaces, extract on the remote server. You can also select the
extracted `<extension-id>/` directory in the extension manager's local-directory
installation flow. No Go toolchain is needed to use a release package; extension
specific tools described above still need to be available on the workspace server.

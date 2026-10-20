# goclip

Cross-platform, pure-Go system clipboard library and `xclip`-compatible CLI utility with automatic environment detection and multi-level fallback chains.

### ⚡ Quick Download (Nightly Builds)

| Platform | Format | Link |
| :--- | :--- | :--- |
| **Windows** | .zip | [amd64](https://github.com/unxed/goclip/releases/download/nightly/goclip-windows-amd64.zip) / [arm64](https://github.com/unxed/goclip/releases/download/nightly/goclip-windows-arm64.zip) |
| **macOS** | .tar.gz | [amd64](https://github.com/unxed/goclip/releases/download/nightly/goclip-darwin-amd64.tar.gz) / [arm64](https://github.com/unxed/goclip/releases/download/nightly/goclip-darwin-arm64.tar.gz) |
| **Linux** | .tar.gz | [amd64](https://github.com/unxed/goclip/releases/download/nightly/goclip-linux-amd64.tar.gz) / [arm64](https://github.com/unxed/goclip/releases/download/nightly/goclip-linux-arm64.tar.gz) / [armv7l](https://github.com/unxed/goclip/releases/download/nightly/goclip-linux-arm.tar.gz) |
| **FreeBSD** | .tar.gz | [amd64](https://github.com/unxed/goclip/releases/download/nightly/goclip-freebsd-amd64.tar.gz) / [arm64](https://github.com/unxed/goclip/releases/download/nightly/goclip-freebsd-arm64.tar.gz) |

*These builds are automated and represent the current state of the `main` branch.*

---

## Features

- **Windows**: Native Win32 API (`OpenClipboard`, `GetClipboardData`, `SetClipboardData`, `CF_UNICODETEXT`) without CGO.
- **macOS**: Native Cocoa/NSPasteboard integration and `pbcopy`/`pbpaste` support.
- **Linux & *nix**:
  1. **Wayland**: Pure-Go Wayland clipboard client / `wl-clipboard` (`wl-copy`, `wl-paste`).
  2. **X11**: Pure-Go X11 client via `github.com/jezek/xgb` (`CLIPBOARD`, `UTF8_STRING`, `TARGETS`) and `xclip`/`xsel`.
  3. **Local File Fallback**: Robust, atomic file-backed clipboard in user configuration directory (far2l-style `FSClipboardBackend`).
- **CLI Utility**: 100% parameter-compatible replacement for `xclip`.
- **Zero-CGO**: Fully compatible with cross-compilation (`CGO_ENABLED=0`).

---

## CLI Utility (`goclip`)

`goclip` can be used as a drop-in replacement for `xclip` across all operating systems.

### Building CLI from source

```bash
git clone https://github.com/unxed/goclip.git
cd goclip
CGO_ENABLED=0 go build -o goclip ./cmd/goclip
```

### CLI Examples

```bash
# Copy text from stdin into clipboard (default mode)
echo "Hello from terminal" | goclip

# Copy text from a file into clipboard
goclip -selection clipboard file.txt

# Paste text from clipboard to stdout
goclip -o

# Paste without trailing newline
goclip -o -rmlastnl

# Copy and pass through to stdout
cat text.txt | goclip -f > output.txt

# Run with diagnostics enabled (troubleshooting)
goclip --debug -o
```

---

## Go Library Usage

```go
package main

import (
    "fmt"
    "github.com/unxed/goclip"
)

func main() {
    // Write text to clipboard
    err := goclip.WriteText("Hello from Go!")
    if err != nil {
        panic(err)
    }

    // Read text from clipboard
    text, err := goclip.ReadText()
    if err != nil {
        panic(err)
    }
    fmt.Println("Clipboard:", text)

    // Convenience functions
    goclip.Set("Quick snippet")
    fmt.Println(goclip.Get())
}
```

---

## Testing Guide

Инструкция для ручного тестирования на русском языке: [TESTING_RU.md](TESTING_RU.md).

## Clipboard images

`ReadImage(ctx)` returns `image.Image`, or `ErrNoImage` when no supported image
is offered, `ErrUnsupportedFormat` for text-only drivers, and a diagnostic error
for failed reads or corrupt data. Existing `Driver` implementations are unchanged;
image support is the optional `ImageReader` interface.

`ReadContents(ctx)` reads an image and the actual accompanying text without the
file fallback. `ContentsReader` allows atomic snapshots; Windows copies both
representations while holding one clipboard lock. Other backends capture their
representations during the same read operation.

Windows accepts registered PNG and uncompressed 24/32-bit DIB/V4/V5 formats.
X11 uses native selection requests with incremental-transfer support. Wayland
uses `wl-paste` MIME negotiation (install wl-clipboard for this backend). macOS
uses the system JXA/AppKit bridge to read PNG or convert TIFF to PNG. Builds stay
pure Go with `CGO_ENABLED=0`. Encoded inputs and decoded dimensions are bounded.

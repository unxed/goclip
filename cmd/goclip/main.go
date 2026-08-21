package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/unxed/goclip"
)

const version = "goclip 1.0 (xclip-compatible clipboard interface in Go)"

const usageText = `Usage: goclip [OPTION] [FILE]...
Access an X/Wayland/system clipboard selection for reading or writing.

  -i, -in          read text into clipboard from standard input or files (default)
  -o, -out         prints the selection to standard out
  -f, -filter      when in write mode, print text back to standard out
  -l, -loops       number of selection requests to wait for (compatibility stub)
  -d, -display     X display to connect to (eg "localhost:0")
  -h, -help, --help  display this help and exit
  -se, -sel, -selection  specify selection ("primary", "secondary", "clipboard", "buffer-cut")
  -noutf8          compatibility flag (ignored, utf-8 supported by default)
  -t, -target      use the given target format
  -rmlastnl        remove the last newline character if present
  -version         output version information and exit
    -silent          run silently in background (compatibility flag)
    -q, -quiet       run in foreground without informational messages
    -v, -verbose     verbose output mode
    --debug, -debug  output detailed diagnostic and environment information
  `

type cliOptions struct {
	modeIn    bool
	modeOut   bool
	filter    bool
	rmLastNL  bool
	verbose   bool
	quiet     bool
	debug     bool
	selection string
	target    string
	display   string
	files     []string
}

func parseCLIArgs(out io.Writer, args []string) (*cliOptions, bool, error) {
	opts := &cliOptions{
		modeIn:    true,
		selection: "primary",
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "-h" || arg == "-help" || arg == "--help" {
			if out != nil {
				fmt.Fprint(out, usageText)
			}
			return nil, true, nil
		}
		if arg == "-version" || arg == "--version" {
			if out != nil {
				fmt.Fprintln(out, version)
			}
			return nil, true, nil
		}

		if arg == "-i" || arg == "-in" {
			opts.modeIn = true
			opts.modeOut = false
			continue
		}
		if arg == "-o" || arg == "-out" {
			opts.modeOut = true
			opts.modeIn = false
			continue
		}
		if arg == "-f" || arg == "-filter" {
			opts.filter = true
			continue
		}
		if arg == "-rmlastnl" {
			opts.rmLastNL = true
			continue
		}
		if arg == "-v" || arg == "-verbose" {
			opts.verbose = true
			continue
		}
		if arg == "-debug" || arg == "--debug" {
			opts.debug = true
			continue
		}
		if arg == "-q" || arg == "-quiet" || arg == "-silent" {
			opts.quiet = true
			continue
		}
		if arg == "-noutf8" {
			continue
		}

		if arg == "-d" || arg == "-display" {
			if i+1 < len(args) {
				opts.display = args[i+1]
				i++
				continue
			}
			return nil, false, fmt.Errorf("option %s requires an argument", arg)
		}

		if arg == "-t" || arg == "-target" {
			if i+1 < len(args) {
				opts.target = args[i+1]
				i++
				continue
			}
			return nil, false, fmt.Errorf("option %s requires an argument", arg)
		}

		if arg == "-se" || arg == "-sel" || arg == "-selection" {
			if i+1 < len(args) {
				opts.selection = normalizeSelectionName(args[i+1])
				i++
				continue
			}
			return nil, false, fmt.Errorf("option %s requires an argument", arg)
		}

		if strings.HasPrefix(arg, "-selection=") {
			opts.selection = normalizeSelectionName(strings.TrimPrefix(arg, "-selection="))
			continue
		}
		if strings.HasPrefix(arg, "-sel=") {
			opts.selection = normalizeSelectionName(strings.TrimPrefix(arg, "-sel="))
			continue
		}

		if strings.HasPrefix(arg, "-l") || strings.HasPrefix(arg, "-loops") {
			if arg == "-l" || arg == "-loops" {
				if i+1 < len(args) {
					i++
				}
			}
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return nil, false, fmt.Errorf("unrecognized option: %s", arg)
		}

		opts.files = append(opts.files, arg)
	}

	return opts, false, nil
}

func normalizeSelectionName(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "c"):
		return "clipboard"
	case strings.HasPrefix(lower, "p"):
		return "primary"
	case strings.HasPrefix(lower, "s"):
		return "secondary"
	case strings.HasPrefix(lower, "b"):
		return "buffer-cut"
	default:
		return lower
	}
}

func runCLI(in io.Reader, out io.Writer, args []string) int {
	opts, exitEarly, err := parseCLIArgs(out, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if exitEarly {
		return 0
	}

	if opts.display != "" {
		_ = os.Setenv("DISPLAY", opts.display)
	}

	if opts.debug {
		printDiagnostics(os.Stderr, opts)
	}

	if opts.modeOut {
		text, err := goclip.ReadText()
		if err != nil {
			if err != goclip.ErrEmpty {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
			text = ""
		}

		if opts.rmLastNL {
			text = strings.TrimSuffix(text, "\n")
			text = strings.TrimSuffix(text, "\r")
		}

		_, _ = io.WriteString(out, text)
		return 0
	}

	// Write mode (Input)
	var content bytes.Buffer
	if len(opts.files) > 0 {
		for _, file := range opts.files {
			data, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading %q: %v\n", file, err)
				return 1
			}
			content.Write(data)
		}
	} else if in != nil {
		_, err := io.Copy(&content, in)
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			return 1
		}
	}

	dataStr := content.String()
	if opts.rmLastNL {
		dataStr = strings.TrimSuffix(dataStr, "\n")
		dataStr = strings.TrimSuffix(dataStr, "\r")
	}

	if err := goclip.WriteText(dataStr); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to clipboard: %v\n", err)
		return 1
	}

	if opts.filter {
		_, _ = io.WriteString(out, dataStr)
	}
	return 0
}

func main() {
	code := runCLI(os.Stdin, os.Stdout, os.Args[1:])
	os.Exit(code)
}
func printDiagnostics(w io.Writer, opts *cliOptions) {
	fmt.Fprintln(w, "=== goclip diagnostics ===")
	fmt.Fprintf(w, "Version:         %s\n", version)
	fmt.Fprintf(w, "OS/Arch:         %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(w, "DISPLAY:         %q\n", os.Getenv("DISPLAY"))
	fmt.Fprintf(w, "WAYLAND_DISPLAY: %q\n", os.Getenv("WAYLAND_DISPLAY"))
	fmt.Fprintf(w, "XDG_DATA_HOME:   %q\n", os.Getenv("XDG_DATA_HOME"))
	fmt.Fprintf(w, "Active driver:   %s\n", goclip.ActiveDriver().Name())
	fmt.Fprintf(w, "Drivers list:    %v\n", goclip.RegisteredDrivers())
	fmt.Fprintf(w, "Mode:            in=%v, out=%v, filter=%v, rmlastnl=%v, sel=%q\n",
		opts.modeIn, opts.modeOut, opts.filter, opts.rmLastNL, opts.selection)
	fmt.Fprintln(w, "==========================")
}

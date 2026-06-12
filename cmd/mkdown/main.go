// main.go — CLI entry point for mkdown, the markdown-to-HTML converter.
//
// Responsible for flag parsing and orchestrating conversions. A single input
// file keeps the original detailed behavior (honors -o, prints the generated
// path). Multiple input files are converted concurrently through a worker pool
// sized to the CPU count — this is where the win for large batches comes from,
// since per-file conversion is CPU-bound and independent. The shared Converter
// is safe for concurrent use: goldmark is reentrant and internal.Convert keeps
// all per-conversion state local (see internal/converter.go).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ekinertac/mkdown/internal"
)

// Build metadata, overridden at release time via -ldflags by goreleaser
// (-X main.version=... etc.). Defaults apply to `go build`/`go install` builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Conversion is allocation-heavy and the process is short-lived, so let the
	// heap grow further between collections to trade some memory for speed.
	// Live memory stays bounded by the number of concurrent workers (not the
	// batch size), so this is safe even for very large batches.
	debug.SetGCPercent(400)

	// Parse flags manually to allow flags after positional args
	var (
		showVersion   bool
		outputPath    string
		inputPaths    []string
		theme         = "dark" // default theme
		enableMermaid bool
		enableMath    bool
		noHighlight   bool
		watch         bool
	)

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-v", "--version":
			showVersion = true
		case "-o", "--output":
			if i+1 < len(os.Args) {
				outputPath = os.Args[i+1]
				i++ // Skip next arg
			} else {
				fmt.Fprintln(os.Stderr, "Error: -o requires an argument")
				os.Exit(1)
			}
		case "-t", "--theme":
			if i+1 < len(os.Args) {
				theme = os.Args[i+1]
				if theme != "dark" && theme != "light" {
					fmt.Fprintf(os.Stderr, "Error: Invalid theme '%s'. Available: dark, light\n", theme)
					os.Exit(1)
				}
				i++ // Skip next arg
			} else {
				fmt.Fprintln(os.Stderr, "Error: -t requires an argument")
				os.Exit(1)
			}
		case "--mermaid":
			enableMermaid = true
		case "--math":
			enableMath = true
		case "--no-highlight":
			noHighlight = true
		case "--watch":
			watch = true
		case "-h", "--help":
			fmt.Println("Usage: mkdown <input.md>... [flags]")
			fmt.Println("\nFlags:")
			fmt.Println("  -o, --output <path>  Output file path (single input only; default: input name with .html)")
			fmt.Println("  -t, --theme <name>   Theme to use: dark (default), light")
			fmt.Println("  --mermaid            Enable Mermaid diagram support (requires internet)")
			fmt.Println("  --math               Enable math rendering with KaTeX (requires internet)")
			fmt.Println("  --no-highlight       Skip syntax highlighting (much faster for bulk/code-heavy docs)")
			fmt.Println("  --watch              Re-render the file on every change (single file; Ctrl+C to stop)")
			fmt.Println("  -v, --version        Show version")
			fmt.Println("  -h, --help          Show this help")
			fmt.Println("\nExamples:")
			fmt.Println("  mkdown README.md")
			fmt.Println("  mkdown input.md -o output.html")
			fmt.Println("  mkdown doc.md --theme light")
			fmt.Println("  mkdown *.md                  # batch: converts every file in parallel")
			fmt.Println("  mkdown diagram.md --mermaid")
			fmt.Println("  mkdown math.md --math")
			fmt.Println("  mkdown --watch README.md     # re-render on save until Ctrl+C")
			os.Exit(0)
		default:
			if !strings.HasPrefix(arg, "-") {
				inputPaths = append(inputPaths, arg)
			} else {
				fmt.Fprintf(os.Stderr, "Error: Unknown flag: %s\n", arg)
				os.Exit(1)
			}
		}
	}

	if showVersion {
		fmt.Printf("mkdown v%s (commit %s, built %s)\n", version, commit, date)
		os.Exit(0)
	}

	if len(inputPaths) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mkdown <input.md>... [-o output.html]")
		fmt.Fprintln(os.Stderr, "Example: mkdown README.md")
		os.Exit(1)
	}

	if outputPath != "" && len(inputPaths) > 1 {
		fmt.Fprintln(os.Stderr, "Error: -o cannot be used with multiple input files")
		os.Exit(1)
	}

	// Validate every input up front so the batch fails fast on a bad path.
	for _, in := range inputPaths {
		if _, err := os.Stat(in); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: File '%s' not found\n", in)
			os.Exit(1)
		}
		lower := strings.ToLower(in)
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".markdown") {
			fmt.Fprintf(os.Stderr, "Error: Input file must be a markdown file (.md or .markdown): %s\n", in)
			os.Exit(1)
		}
	}

	converter := internal.NewConverterWithOptions(internal.ConverterOptions{
		Theme:            theme,
		EnableMermaid:    enableMermaid,
		EnableMath:       enableMath,
		DisableHighlight: noHighlight,
	})

	// Watch mode: re-render a single file on every change until interrupted.
	if watch {
		if len(inputPaths) != 1 {
			fmt.Fprintln(os.Stderr, "Error: --watch requires exactly one input file")
			os.Exit(1)
		}
		out := outputPath
		if out == "" {
			out = outputFor(inputPaths[0])
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := internal.Watch(ctx, converter, inputPaths[0], out, 250*time.Millisecond); err != nil {
			stop() // release the signal handler before exiting (os.Exit skips defers)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Single file: keep the original, chatty behavior.
	if len(inputPaths) == 1 {
		out := outputPath
		if out == "" {
			out = outputFor(inputPaths[0])
		}
		if err := converter.Convert(inputPaths[0], out); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Generated: %s (theme: %s%s)\n", out, theme, featureSuffix(enableMermaid, enableMath))
		return
	}

	// Batch: convert all inputs concurrently with a bounded worker pool.
	runBatch(converter, inputPaths, theme, enableMermaid, enableMath)
}

// runBatch converts many files in parallel and prints a summary. The converter
// is shared across workers; each Convert call is self-contained.
func runBatch(converter *internal.Converter, inputPaths []string, theme string, mermaid, math bool) {
	// Size the pool to GOMAXPROCS, not NumCPU. GOMAXPROCS (Go 1.25+) respects
	// the cgroup CPU quota, so in a CPU-limited container or CI runner we don't
	// oversubscribe the few cores we actually have with host-core-count workers;
	// it also honors an explicit GOMAXPROCS override. On an unconstrained
	// machine the two are equal.
	workers := runtime.GOMAXPROCS(0)
	if workers > len(inputPaths) {
		workers = len(inputPaths)
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	var failed int32

	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for in := range jobs {
				if err := converter.Convert(in, outputFor(in)); err != nil {
					fmt.Fprintf(os.Stderr, "Error converting %s: %v\n", in, err)
					atomic.AddInt32(&failed, 1)
				}
			}
		}()
	}
	for _, in := range inputPaths {
		jobs <- in
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(start)
	ok := len(inputPaths) - int(failed)
	fmt.Printf("✓ Converted %d/%d files in %s (theme: %s%s, %d workers)\n",
		ok, len(inputPaths), elapsed.Round(time.Millisecond), theme, featureSuffix(mermaid, math), workers)
	if failed > 0 {
		os.Exit(1)
	}
}

// outputFor maps an input markdown path to its sibling .html output path.
func outputFor(inputPath string) string {
	ext := filepath.Ext(inputPath)
	return strings.TrimSuffix(inputPath, ext) + ".html"
}

func featureSuffix(mermaid, math bool) string {
	var features []string
	if mermaid {
		features = append(features, "mermaid")
	}
	if math {
		features = append(features, "math")
	}
	if len(features) == 0 {
		return ""
	}
	return fmt.Sprintf(" [%s]", strings.Join(features, ", "))
}

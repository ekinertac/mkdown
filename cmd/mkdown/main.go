package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ekinertac/mkdown/internal"
)

const version = "0.1.0"

func main() {
	// Parse flags manually to allow flags after positional args
	var (
		showVersion   bool
		outputPath    string
		inputPath     string
		theme         = "dark" // default theme
		enableMermaid bool
		enableMath    bool
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
		case "-h", "--help":
			fmt.Println("Usage: mkdown <input.md> [flags]")
			fmt.Println("\nFlags:")
			fmt.Println("  -o, --output <path>  Output file path (default: input file name with .html extension)")
			fmt.Println("  -t, --theme <name>   Theme to use: dark (default), light")
			fmt.Println("  --mermaid            Enable Mermaid diagram support (requires internet)")
			fmt.Println("  --math               Enable math rendering with KaTeX (requires internet)")
			fmt.Println("  -v, --version        Show version")
			fmt.Println("  -h, --help          Show this help")
			fmt.Println("\nExamples:")
			fmt.Println("  mkdown README.md")
			fmt.Println("  mkdown input.md -o output.html")
			fmt.Println("  mkdown doc.md --theme light")
			fmt.Println("  mkdown diagram.md --mermaid")
			fmt.Println("  mkdown math.md --math")
			fmt.Println("  mkdown doc.md --mermaid --math --theme light")
			os.Exit(0)
		default:
			if !strings.HasPrefix(arg, "-") && inputPath == "" {
				inputPath = arg
			} else if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Error: Unknown flag: %s\n", arg)
				os.Exit(1)
			}
		}
	}

	if showVersion {
		fmt.Printf("mkdown v%s\n", version)
		os.Exit(0)
	}

	if inputPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: mkdown <input.md> [-o output.html]")
		fmt.Fprintln(os.Stderr, "Example: mkdown README.md")
		os.Exit(1)
	}

	// Validate input file exists and is markdown
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: File '%s' not found\n", inputPath)
		os.Exit(1)
	}

	if !strings.HasSuffix(strings.ToLower(inputPath), ".md") && 
	   !strings.HasSuffix(strings.ToLower(inputPath), ".markdown") {
		fmt.Fprintf(os.Stderr, "Error: Input file must be a markdown file (.md or .markdown)\n")
		os.Exit(1)
	}

	// Determine output path
	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		outputPath = strings.TrimSuffix(inputPath, ext) + ".html"
	}

	// Convert
	converter := internal.NewConverterWithOptions(internal.ConverterOptions{
		Theme:         theme,
		EnableMermaid: enableMermaid,
		EnableMath:    enableMath,
	})
	if err := converter.Convert(inputPath, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var features []string
	if enableMermaid {
		features = append(features, "mermaid")
	}
	if enableMath {
		features = append(features, "math")
	}

	featureStr := ""
	if len(features) > 0 {
		featureStr = fmt.Sprintf(" [%s]", strings.Join(features, ", "))
	}

	fmt.Printf("✓ Generated: %s (theme: %s%s)\n", outputPath, theme, featureStr)
}


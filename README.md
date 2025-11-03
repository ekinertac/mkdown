# mkdown

A simple, fast markdown to HTML converter with a clean default template.

```bash
mkdown input.md              # Converts to input.html
mkdown doc.md -o out.html   # Custom output
```

## Features

- Single binary, no dependencies
- GitHub Flavored Markdown support (tables, strikethrough, task lists)
- Syntax highlighting with Chroma
- Frontmatter parsing (YAML)
- Dark theme by default (light theme available)
- Separated CSS for easy theming

## Installation

```bash
go install github.com/ekinertac/mkdown@latest
```

Or build from source:

```bash
git clone https://github.com/ekinertac/mkdown.git
cd mkdown
go build

# Run tests
make test

# Or with coverage
make test-coverage
```

## Usage

### Basic Usage

Convert a markdown file to HTML:

```bash
mkdown input.md
```

This creates `input.html` in the same directory.

### CLI Flags

```
mkdown <input.md> [flags]

Flags:
  -o, --output <path>  Output file path (default: input filename with .html extension)
  -t, --theme <name>   Theme to use: dark (default), light
  --mermaid            Enable Mermaid diagram support (requires internet)
  --math               Enable math rendering with KaTeX (requires internet)
  -v, --version        Show version number
  -h, --help          Show help message

Examples:
  mkdown README.md                          # Creates README.html (dark theme)
  mkdown doc.md -o output.html             # Custom output path
  mkdown doc.md --theme light              # Use light theme
  mkdown diagram.md --mermaid              # Enable Mermaid diagrams
  mkdown math.md --math                    # Enable math rendering
  mkdown doc.md --mermaid --math --theme light  # All features
```

### Configuration

Config file support (via `~/.mkdown.yml`) is planned for future releases. This will allow:

- Custom theme selection
- Default output paths
- Extension preferences

## Frontmatter

Add metadata to your markdown files:

```markdown
---
title: My Document
author: John Doe
---

# Content starts here
```

The `title` field will be used as the HTML page title.

## Extensions

### Phase 1 (Complete ✅)

- **Tables**: GitHub-style tables
- **Strikethrough**: `~~text~~`
- **Task Lists**: `- [ ]` and `- [x]`
- **Syntax Highlighting**: Fenced code blocks with language tags
- **Auto Heading IDs**: For anchor links
- **Footnotes**: `[^1]` reference style footnotes
- **Definition Lists**: Term and definition pairs
- **Typographer**: Smart quotes, em/en dashes, ellipsis
- **Linkify**: Auto-convert URLs to clickable links

See `examples/extensions.md` for usage examples.

### Phase 2 (Complete ✅)

- **Mermaid Diagrams**: Flowcharts, sequence diagrams, gantt charts (use `--mermaid` flag)
- **Math Rendering**: LaTeX-style equations with KaTeX (use `--math` flag)

See `examples/mermaid-demo.md` and `examples/math-demo.md` for examples.

## Examples

See `examples/` directory for sample markdown files:

- **`showcase.md`** - Complete feature demonstration (all phases)
- **`extensions.md`** - Phase 1 features (footnotes, definition lists, etc.)
- **`mermaid-demo.md`** - Mermaid diagram examples
- **`math-demo.md`** - Math equation examples
- **`combined-demo.md`** - Mermaid + Math together
- **`sample.md`** - Basic markdown features

**Quick start:**

```bash
mkdown examples/showcase.md --mermaid --math
open examples/showcase.html
```

## Theming

mkdown uses a clean architecture with separated CSS:

- **Template**: `internal/templates/default.html` (minimal HTML structure)
- **Dark theme**: `internal/templates/dark.css` (default, GitHub dark palette)
- **Light theme**: `internal/templates/light.css` (GitHub light palette)

### Using Themes

```bash
# Dark theme (default)
mkdown input.md

# Light theme
mkdown input.md --theme light

# Short flag
mkdown input.md -t light
```

Both themes include:

- GitHub-style typography and spacing
- Syntax highlighting (Monokai for dark, GitHub Light for light)
- Responsive tables, lists, and blockquotes

**Planned features**:

- Theme configuration via `~/.mkdown.yml`
- Custom theme support (bring your own CSS)

## Project Structure

See [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) for detailed folder organization and architecture.

## License

MIT

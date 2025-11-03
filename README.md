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
  -v, --version        Show version number
  -h, --help          Show help message

Examples:
  mkdown README.md                    # Creates README.html (dark theme)
  mkdown doc.md -o output.html       # Custom output path
  mkdown doc.md --theme light        # Use light theme
  mkdown doc.md -t light -o out.html # Light theme + custom output
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

- **Tables**: GitHub-style tables
- **Strikethrough**: `~~text~~`
- **Task Lists**: `- [ ]` and `- [x]`
- **Syntax Highlighting**: Fenced code blocks with language tags
- **Auto Heading IDs**: For anchor links

## Examples

See `examples/` directory for sample markdown files.

## Theming

mkdown uses a clean architecture with separated CSS:

- **Template**: `templates/default.html` (minimal HTML structure)
- **Dark theme**: `templates/dark.css` (default, GitHub dark palette)
- **Light theme**: `templates/light.css` (GitHub light palette)

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

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for technical details.

## License

MIT

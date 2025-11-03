# Project Structure

```
mkdown/
├── cmd/
│   └── mkdown/              # Main application
│       ├── main.go          # CLI entry point
│       └── main_test.go     # Integration tests
│
├── internal/                # Private application code
│   ├── converter.go         # Core conversion logic
│   ├── converter_test.go    # Unit tests
│   └── templates/           # Embedded templates & CSS
│       ├── default.html     # HTML template (13 lines)
│       ├── dark.css         # Dark theme styles
│       └── light.css        # Light theme styles
│
├── examples/                # Sample markdown files
│   ├── sample.md           # Basic features demo
│   ├── extensions.md       # Extension examples
│   └── theme-comparison.md # Theme showcase
│
├── go.mod                  # Go module definition
├── go.sum                  # Dependency checksums
├── Makefile               # Build & test commands
├── README.md              # Main documentation
├── EXTENSIONS.md          # Extension roadmap
├── LICENSE                # MIT license
└── .gitignore            # Git ignore rules
```

## Directory Explanations

### `cmd/mkdown/`
Contains the main application entry point. Following Go convention, executable commands live in `cmd/`.

- **main.go**: CLI argument parsing, file validation, orchestration
- **main_test.go**: End-to-end integration tests

### `internal/`
Private application code that cannot be imported by external packages.

- **converter.go**: Core markdown → HTML conversion using goldmark
- **converter_test.go**: Unit tests for conversion logic
- **templates/**: Embedded assets (via `//go:embed`)

### Why This Structure?

1. **Standard Go layout**: Follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout)
2. **Separation of concerns**: CLI logic separate from conversion logic
3. **Testability**: Internal package can be unit tested independently
4. **Encapsulation**: `internal/` prevents external imports
5. **Clean root**: Documentation and config files only at root

## Building

```bash
# Build binary
go build -o mkdown ./cmd/mkdown

# Or use Makefile
make build
```

## Testing

```bash
# Run all tests
go test ./...

# With coverage
make test-coverage
```

## Adding New Features

### New Extension
1. Add to `internal/converter.go` → `NewConverter()`
2. Add tests to `internal/converter_test.go`
3. Add example to `examples/extensions.md`
4. Update `EXTENSIONS.md`

### New CLI Flag
1. Add flag parsing to `cmd/mkdown/main.go`
2. Add integration test to `cmd/mkdown/main_test.go`
3. Update help text
4. Update `README.md`

### New Theme
1. Add CSS file to `internal/templates/`
2. Add `//go:embed` directive in `converter.go`
3. Update theme selection logic
4. Add tests


---
title: Mermaid Diagram Examples
---

# Mermaid Diagram Examples

This document demonstrates Mermaid diagram support.

## Flowchart

```mermaid
graph TD
    A[Start] --> B{Is it working?}
    B -->|Yes| C[Great!]
    B -->|No| D[Debug]
    D --> A
    C --> E[End]
```

## Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant mkdown
    participant Browser
    
    User->>mkdown: Convert markdown
    mkdown->>mkdown: Parse & render
    mkdown->>Browser: Generate HTML
    Browser->>User: Display diagrams
```

## Class Diagram

```mermaid
classDiagram
    class Converter {
        +markdown goldmark.Markdown
        +template *template.Template
        +theme string
        +Convert(input, output)
    }
    
    class Document {
        +Title string
        +Content HTML
        +Styles CSS
        +Scripts HTML
    }
    
    Converter --> Document : creates
```

## Gantt Chart

```mermaid
gantt
    title mkdown Development Roadmap
    dateFormat YYYY-MM-DD
    section Phase 1
    Basic Conversion    :done, 2025-11-01, 2d
    Extensions          :done, 2025-11-02, 1d
    section Phase 2
    Mermaid Support     :active, 2025-11-03, 1d
    Math Support        :2025-11-03, 1d
    section Phase 3
    Custom Containers   :2025-11-04, 2d
```

## Pie Chart

```mermaid
pie title Languages in Example Codebase
    "Go" : 70
    "CSS" : 20
    "JavaScript" : 5
    "HTML" : 5
```

---

## Fullscreen Mode

Each diagram has a fullscreen button (⛶) in the top-right corner. Click it to:
- Expand diagram to full screen
- View large diagrams without scrolling
- Close with Escape key or click the ✕ button

Perfect for complex flowcharts, large sequence diagrams, or detailed class diagrams!

---

**Note**: Mermaid diagrams require internet connection to load the Mermaid.js library from CDN.

To generate this file:
```bash
mkdown mermaid-demo.md --mermaid
```


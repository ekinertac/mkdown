---
title: Combined Features Demo
---

# Combined Features Demo

This document demonstrates using Mermaid diagrams and math equations together.

## System Architecture with Math

The system processes requests with complexity $O(\log n)$ where $n$ is the number of items:

```mermaid
graph LR
    A[Input] --> B[Parser]
    B --> C{Valid?}
    C -->|Yes| D[Renderer]
    C -->|No| E[Error Handler]
    D --> F[Output]
```

The time complexity for conversion is approximately:

$$
T(n) = O(n \cdot \log n)
$$

Where $n$ is the document size in bytes.

## Algorithm Flow

```mermaid
sequenceDiagram
    participant Client
    participant Converter
    participant Goldmark
    participant Template
    
    Client->>Converter: Convert(input)
    Converter->>Goldmark: Parse markdown
    Note over Goldmark: O(n) complexity
    Goldmark-->>Converter: AST
    Converter->>Template: Render(doc)
    Template-->>Converter: HTML
    Converter-->>Client: output.html
```

The rendering pipeline has constant space complexity:

$$
S(n) = O(1)
$$

## Performance Metrics

```mermaid
pie title Processing Time Distribution
    "Parsing" : 45
    "Rendering" : 30
    "I/O" : 15
    "Other" : 10
```

Average processing speed: $\approx 1.2 \text{ MB/s}$

## Feature Matrix

| Feature | Complexity | Enabled |
|---------|------------|---------|
| Tables | $O(n \cdot m)$ | ✓ |
| Footnotes | $O(n)$ | ✓ |
| Math | $O(k)$ | ✓ |
| Diagrams | $O(v + e)$ | ✓ |

Where:
- $n$ = number of rows
- $m$ = number of columns  
- $k$ = number of equations
- $v$ = vertices, $e$ = edges

## Data Flow

```mermaid
flowchart TB
    subgraph Input
        MD[Markdown File]
        FM[Frontmatter]
    end
    
    subgraph Processing
        P[Parser]
        R[Renderer]
        S[Script Injector]
    end
    
    subgraph Output
        HTML[HTML File]
        CSS[Embedded CSS]
        JS[CDN Scripts]
    end
    
    MD --> P
    FM --> P
    P --> R
    R --> S
    S --> HTML
    CSS --> HTML
    JS --> HTML
```

Output size estimation:

$$
\text{Size}_{\text{output}} = \text{Size}_{\text{input}} \times 1.5 + \text{Size}_{\text{CSS}} + \text{Size}_{\text{JS}}
$$

---

Generate this with all features:
```bash
mkdown combined-demo.md --mermaid --math
```


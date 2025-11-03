---
title: Extension Examples
---

# Extension Examples

This document demonstrates all Phase 1 extensions available in mkdown.

## Footnotes

Here's a sentence with a footnote reference.[^1] You can also have multiple footnotes.[^2]

Footnotes can contain multiple paragraphs[^long] and even code blocks.

[^1]: This is a simple footnote.
[^2]: This is another footnote with **bold text**.
[^long]: This is a longer footnote.

    It contains multiple paragraphs and code:
    
    ```go
    fmt.Println("in a footnote!")
    ```

## Definition Lists

Apple
: A fruit that grows on trees
: Available in many varieties

Markdown
: A lightweight markup language
: Created by John Gruber in 2004

API
: Application Programming Interface
: Allows software to communicate

## Typographer

### Smart Quotes

Use "double quotes" and 'single quotes' -- they'll be converted to smart quotes.

### Dashes

- Two hyphens -- become an en-dash
- Three hyphens --- become an em-dash

### Other

- Three dots... become an ellipsis
- Fractions: 1/2, 1/4, 3/4 (depends on font support)
- French quotes: <<hello>> become guillemets

## Auto Linkify

Plain URLs are automatically converted to links:

Visit https://github.com/ekinertac/mkdown for the source code.

Email addresses work too: contact@example.com

## Combined Example

Here's a practical example combining multiple extensions:

HTTP Status Codes
: 200 -- OK
: 404 -- Not Found
: 500 -- Internal Server Error

You can read more about REST APIs[^rest] at https://restfulapi.net

"The best way to predict the future is to invent it." --- Alan Kay

[^rest]: REST stands for Representational State Transfer. It's an architectural style for designing networked applications.

## Tables with Footnotes

| Feature | Status | Notes |
|---------|--------|-------|
| Footnotes | ✓ | See above[^fn-support] |
| Typographer | ✓ | Smart quotes, dashes... |
| Definition Lists | ✓ | Great for glossaries |
| Linkify | ✓ | Auto URL conversion |

[^fn-support]: Footnotes are rendered at the bottom of the document automatically.

---

All extensions work together seamlessly!


```
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
```

## Common Commands
- **Build**: `go build` (compiles the Go application)
- **Run**: `go run main.go` (starts the Go web server)
- **Test**: `go test` (runs unit tests; no test files found in current structure)

## High-Level Architecture
- **Backend**: Go web server (main.go) using standard library packages (e.g., net/http, html/template)
- **Frontend**: Server-rendered HTML templates (templates/index.html, templates/login.html) styled with Pico CSS (css/ folder contains multiple theme variants)
- **Static Assets**: CSS files (css/), images/screen recordings (assets/), and data.txt (purpose unclear)
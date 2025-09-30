# Personal Hub - Architecture Overview

## Project Overview

Personal Hub is a simple Go web application for uploading and managing personal content including text, images, and videos. It features a clean web interface with drag-and-drop file upload capabilities, session-based authentication, and content management.

**Architecture**: Monolithic Go web server using standard library packages with server-side rendered HTML templates.

**Key Features**:
- Session-based authentication with environment variable credentials
- Multi-file upload with drag-and-drop support
- Content type detection (text, images, videos)
- File storage in local `assets/` directory
- Responsive web interface using Pico CSS framework

## Build & Commands

```bash
# Build the application
go build

# Run the development server
go run main.go

# Run tests (no test files currently exist)
go test
```

**Server Configuration**:
- Default port: `:3000`
- Base path: `/hub/`
- Static assets served from `/assets/`

## Code Style

**Go Conventions**:
- Standard Go formatting (gofmt)
- Package structure follows Go conventions
- Error handling with explicit error returns
- HTTP handlers follow standard library patterns

**Frontend**:
- HTML5 semantic markup
- Pico CSS framework for styling
- Vanilla JavaScript for client-side functionality
- Responsive design principles

## Testing

**Current State**: No test files exist in the codebase

**Recommended Testing Approach**:
- Unit tests for core business logic
- HTTP handler tests using `net/http/httptest`
- Integration tests for file upload functionality
- Session management tests

## Security

**Authentication**:
- Environment-based credentials (`MY_HUB_ADMIN_USER`, `MY_ADMIN_PASS`)
- Session-based authentication using Gorilla Sessions
- Constant-time comparison for password validation

**Security Considerations**:
- ⚠️ **WARNING**: Hardcoded secret key in production code (main.go:37)
- ⚠️ **WARNING**: Session cookies not marked as Secure in production
- File upload validation needed (type, size limits)
- Path traversal protection for file uploads
- Input sanitization for text content

**Recommendations**:
- Use secure random secret keys
- Implement proper file type validation
- Add CSRF protection
- Use HTTPS in production
- Implement rate limiting for uploads

## Configuration

**Environment Variables**:
- `MY_HUB_ADMIN_USER`: Admin username
- `MY_ADMIN_PASS`: Admin password

**File Structure**:
- `main.go`: Main application server
- `templates/`: HTML templates (index.html, login.html)
- `assets/`: Uploaded files storage
- `data.txt`: Text content persistence
- `css/`: Pico CSS framework variants

**Dependencies**:
- `github.com/gorilla/sessions v1.4.0`: Session management
- Go 1.23+ required

## Development Notes

**Recent Features** (from git history):
- Drag-and-drop file upload support
- Multiple file selection
- Paste image functionality
- File upload source tracking

**Frontend Features**:
- Drag-and-drop zone with visual feedback
- File list preview with remove functionality
- Keyboard shortcuts (Ctrl/Cmd+A for text selection)
- Clipboard paste support for images

**Data Persistence**:
- Text content stored in `data.txt`
- Files stored in `assets/` directory
- Session-based content tracking
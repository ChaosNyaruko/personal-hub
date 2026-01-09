# Personal Hub - Architecture Overview

## Agent Co-Authors

**Coco** - AI Assistant for code improvements and bug fixes
- Co-author of memory usage optimization (removed in-memory storage, implemented append-only file operations)
- Use `Co-authored-by: Coco <coco@example.com>` in commits when AI assistance is provided

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

# Run tests
go test

# Run tests with verbose output
go test -v

# Run tests and update coverage badge
./update_coverage.sh
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

**Current State**:
- Unit tests exist in `main_test.go` covering core handlers (`loginHandler`, `uploadHandler`, `indexHandler`) and middleware (`authMiddleware`).
- Test coverage is tracked via badge in README.md (currently ~58%).
- `update_coverage.sh` script available to run tests and update the coverage badge.

**Testing Requirements**:
- **Mandatory**: Every new feature or meaningful change MUST be accompanied by corresponding unit tests.
- Existing tests must pass before merging/committing.
- Maintain or improve the current test coverage.

**Testing Strategy**:
- `httptest` is used for testing HTTP handlers.
- `assetsDir` variable allows swapping the storage directory during testing to prevent polluting real data.
- Global variables (`dataFile`, `store`) are mocked/swapped in tests.

**Future Testing Needs**:
- Integration tests for full user flows.
- Browser-based end-to-end testing.

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
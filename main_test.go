package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
)

func TestMain(m *testing.M) {
	// Setup
	var err error
	tpl, err = template.ParseGlob("templates/*.html")
	if err != nil {
		fmt.Printf("Error parsing templates: %v\n", err)
		os.Exit(1)
	}

	// Use a secure key for testing
	store = sessions.NewCookieStore([]byte("test-secret-key"))

	os.Exit(m.Run())
}

func setupTestEnv(t *testing.T) (string, string) {
	tempDir, err := os.MkdirTemp("", "hub_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	testDataFile := filepath.Join(tempDir, "data.txt")
	testAssetsDir := filepath.Join(tempDir, "assets")
	
	if err := os.Mkdir(testAssetsDir, 0755); err != nil {
		t.Fatalf("Failed to create assets dir: %v", err)
	}

	// Override global variables
	dataFile = testDataFile
	assetsDir = testAssetsDir

	return tempDir, testAssetsDir
}

func TestLoginHandler(t *testing.T) {
	// Set admin credentials
	os.Setenv("MY_HUB_ADMIN_USER", "admin")
	os.Setenv("MY_ADMIN_PASS", "password")
	defer os.Unsetenv("MY_HUB_ADMIN_USER")
	defer os.Unsetenv("MY_ADMIN_PASS")

	tests := []struct {
		name           string
		method         string
		username       string
		password       string
		expectedStatus int
		expectedBody   string
		expectAuth     bool
	}{
		{
			name:           "GET Request",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody:   "Login", // Assuming "Login" is in the template
			expectAuth:     false,
		},
		{
			name:           "POST Valid Credentials",
			method:         "POST",
			username:       "admin",
			password:       "password",
			expectedStatus: http.StatusSeeOther, // Redirect
			expectAuth:     true,
		},
		{
			name:           "POST Invalid Username",
			method:         "POST",
			username:       "wrong",
			password:       "password",
			expectedStatus: http.StatusOK, // Rerenders login page
			expectedBody:   "Invalid credentials",
			expectAuth:     false,
		},
		{
			name:           "POST Invalid Password",
			method:         "POST",
			username:       "admin",
			password:       "wrong",
			expectedStatus: http.StatusOK,
			expectedBody:   "Invalid credentials",
			expectAuth:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.method == "POST" {
				form := strings.NewReader(fmt.Sprintf("username=%s&password=%s", tt.username, tt.password))
				req = httptest.NewRequest("POST", "/login", form)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest("GET", "/login", nil)
			}

			w := httptest.NewRecorder()
			loginHandler(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedBody != "" {
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), tt.expectedBody) {
					t.Errorf("expected body to contain %q, got %q", tt.expectedBody, string(body))
				}
			}

			if tt.expectAuth {
				// Check session
				session, _ := store.Get(req, "session")
				if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
					t.Error("expected session to be authenticated")
				}
			}
		})
	}
}

func TestUploadHandler_Text(t *testing.T) {
	tempDir, _ := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("text", "Hello World")
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	w := httptest.NewRecorder()
	uploadHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}

	// Verify file content
	content, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("Failed to read data file: %v", err)
	}
	if !strings.Contains(string(content), "Hello World") {
		t.Errorf("expected file to contain 'Hello World', got %q", string(content))
	}
}

func TestUploadHandler_File(t *testing.T) {
	tempDir, testAssetsDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.png")
	part.Write([]byte("file content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	uploadHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}

	// Verify file exists
	uploadedFile := filepath.Join(testAssetsDir, "test.png")
	if _, err := os.Stat(uploadedFile); os.IsNotExist(err) {
		t.Error("expected uploaded file to exist")
	}
	content, _ := os.ReadFile(uploadedFile)
	if string(content) != "file content" {
		t.Errorf("expected file content 'file content', got %q", string(content))
	}
}

func TestUploadHandler_InvalidExtension(t *testing.T) {
	tempDir, testAssetsDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "malicious.exe")
	part.Write([]byte("malicious content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	uploadHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}

	// Verify file DOES NOT exist
	uploadedFile := filepath.Join(testAssetsDir, "malicious.exe")
	if _, err := os.Stat(uploadedFile); !os.IsNotExist(err) {
		t.Error("expected invalid file to NOT be uploaded")
	}
}

func TestIndexHandler_Sorting(t *testing.T) {
	tempDir, testAssetsDir := setupTestEnv(t)
	defer os.RemoveAll(tempDir)

	// Create test data
	// 1. Text data: older then newer
	f, _ := os.Create(dataFile)
	fmt.Fprintln(f, "Old Text")
	fmt.Fprintln(f, "New Text")
	f.Close()

	// 2. File data: older file then newer file
	os.WriteFile(filepath.Join(testAssetsDir, "old.png"), []byte("old"), 0644)
	time.Sleep(100 * time.Millisecond) // Ensure time difference
	os.WriteFile(filepath.Join(testAssetsDir, "new.jpg"), []byte("new"), 0644)
	
	// Update modification times to be sure
	now := time.Now()
	os.Chtimes(filepath.Join(testAssetsDir, "new.jpg"), now, now)
	os.Chtimes(filepath.Join(testAssetsDir, "old.png"), now.Add(-1*time.Hour), now.Add(-1*time.Hour))

	// Mock authenticated request
	req := httptest.NewRequest("GET", "/", nil)
	session, _ := store.Get(req, "session")
	session.Values["authenticated"] = true
	
	// Create a cookie with the session
	w := httptest.NewRecorder()
	session.Save(req, w)
	
	// Add the cookie to the request
	req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

	w = httptest.NewRecorder()
	indexHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// We can't easily parse the HTML output without a parser, but we can check if "New Text" appears before "Old Text"?
	// The template renders the slice in order.
	// So "New Text" should appear in the HTML before "Old Text" if it's reversed correctly?
	// Wait, the logic is: "Reverse text content to show newest first".
	// data.txt has:
	// Old Text
	// New Text
	//
	// Reader reads them in order. Slice: ["Old Text", "New Text"]
	// Reversed: ["New Text", "Old Text"]
	// So "New Text" comes first.
	
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	newTextIndex := strings.Index(bodyStr, "New Text")
	oldTextIndex := strings.Index(bodyStr, "Old Text")
	
	if newTextIndex == -1 || oldTextIndex == -1 {
		t.Fatal("expected text content not found in response")
	}

	if newTextIndex > oldTextIndex {
		t.Error("expected 'New Text' to appear before 'Old Text'")
	}

	// Check file sorting
	// "new.jpg" (newer) should appear before "old.png" (older)
	newFileIndex := strings.Index(bodyStr, "new.jpg")
	oldFileIndex := strings.Index(bodyStr, "old.png")

	if newFileIndex == -1 || oldFileIndex == -1 {
		t.Fatal("expected file names not found in response")
	}

	if newFileIndex > oldFileIndex {
		t.Error("expected 'new.jpg' to appear before 'old.png'")
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Mock handler
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}
	
	handler := authMiddleware(mockHandler)

	// Case 1: Unauthenticated
	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("expected redirect for unauthenticated user, got %d", w.Result().StatusCode)
	}

	// Case 2: Authenticated
	req = httptest.NewRequest("GET", "/protected", nil)
	session, _ := store.Get(req, "session")
	session.Values["authenticated"] = true
	// Trick: To persist session in request context for the middleware to see it without a roundtrip,
	// we rely on the store.Get reading from the cookie.
	// We need to inject the cookie.
	rec := httptest.NewRecorder()
	session.Save(req, rec)
	req.Header.Set("Cookie", rec.Header().Get("Set-Cookie"))

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for authenticated user, got %d", w.Result().StatusCode)
	}
}

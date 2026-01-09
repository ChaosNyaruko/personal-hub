package main

import (
	"bufio"
	"crypto/subtle"
	"embed"
	"encoding/gob"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorilla/sessions"
)

type uploadedContent struct {
	FileType   string // "text", "image", or "video"
	Content    string
	MimeType   string
	LinkTarget string
}

type PageData struct {
	Texts []uploadedContent
	Media []uploadedContent
}

var (
	//go:embed templates/*.html
	templateFS embed.FS

	tpl      *template.Template
	store    *sessions.CookieStore
	dataFile = "data.txt"
	assetsDir = "assets"
)

func init() {
	secretKey := os.Getenv("MY_HUB_SECRET_KEY")
	if secretKey == "" {
		log.Println("WARNING: MY_HUB_SECRET_KEY not set. Using default insecure key. Sessions will be insecure!")
		secretKey = "secret-key"
	}

	secureCookie := false
	if os.Getenv("MY_HUB_COOKIE_SECURE") == "true" {
		secureCookie = true
	}

	store = sessions.NewCookieStore([]byte(secretKey))
	store.Options = &sessions.Options{
		Path:     "/",
		Secure:   secureCookie,
		HttpOnly: true,
		MaxAge:   7 * 24 * 3600,
		SameSite: http.SameSiteLaxMode,
	}
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("get wd error: %v", err)
	}
	log.Printf("cwd: %v", cwd)
	gob.Register([]uploadedContent{})
	tpl = template.Must(template.ParseFS(templateFS, "templates/*.html"))

	hub := http.NewServeMux()

	hub.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))
	hub.HandleFunc("/", indexHandler)
	hub.HandleFunc("/login", loginHandler)
	hub.HandleFunc("/logout", logoutHandler)
	hub.HandleFunc("/upload", authMiddleware(uploadHandler))

	http.Handle("/hub/", http.StripPrefix("/hub", hub))

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("Error loading data: %v", err)
	}
}


func authMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "session")
		if err != nil {
			http.Error(w, fmt.Errorf("An error occurred, please try again later %v", err).Error(), http.StatusInternalServerError)
			return
		}
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Redirect(w, r, "/hub/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tpl.ExecuteTemplate(w, "login.html", nil)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	adminUser := os.Getenv("MY_HUB_ADMIN_USER")
	adminPass := os.Getenv("MY_ADMIN_PASS")

	if username == "" || password == "" {
		tpl.ExecuteTemplate(w, "login.html", "Invalid credentials")
		return
	}

	if subtle.ConstantTimeCompare([]byte(username), []byte(adminUser)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(adminPass)) == 1 {
		session, err := store.Get(r, "session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		session.Values["authenticated"] = true
		err = session.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/hub/", http.StatusSeeOther)
		return
	}

	tpl.ExecuteTemplate(w, "login.html", "Invalid credentials")
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["authenticated"] = false
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/hub/login", http.StatusSeeOther)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	session, err := store.Get(r, "session")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/hub/login", http.StatusSeeOther)
		return
	}
	
	// Read content from files instead of session
	var pageData PageData
	
	// Read text content from data.txt
	if file, err := os.Open(dataFile); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		var textContent []uploadedContent
		for scanner.Scan() {
			textContent = append(textContent, uploadedContent{
				FileType: "text", 
				Content:  scanner.Text(), 
				MimeType: "",
			})
		}
		// Reverse text content to show newest first
		for i, j := 0, len(textContent)-1; i < j; i, j = i+1, j-1 {
			textContent[i], textContent[j] = textContent[j], textContent[i]
		}
		pageData.Texts = textContent
	}
	
	// Read files from assets directory
	if files, err := os.ReadDir(assetsDir); err == nil {
		type fileWithInfo struct {
			entry os.DirEntry
			info  os.FileInfo
		}
		var sortedFiles []fileWithInfo

		for _, file := range files {
			if !file.IsDir() {
				info, err := file.Info()
				if err == nil {
					sortedFiles = append(sortedFiles, fileWithInfo{entry: file, info: info})
				}
			}
		}

		// Sort by modification time descending
		sort.Slice(sortedFiles, func(i, j int) bool {
			return sortedFiles[i].info.ModTime().After(sortedFiles[j].info.ModTime())
		})

		for _, item := range sortedFiles {
			file := item.entry
			filename := file.Name()
			fileType := "image"
			mimeType := ""
			ext := strings.ToLower(filepath.Ext(filename))
			
			if ext == ".mp4" || ext == ".webm" || ext == ".ogg" || ext == ".mov" {
				fileType = "video"
				switch ext {
				case ".mp4":
					mimeType = "video/mp4"
				case ".webm":
					mimeType = "video/webm"
				case ".ogg":
					mimeType = "video/ogg"
				case ".mov":
					mimeType = "video/quicktime"
				}
			} else {
				switch ext {
				case ".jpg", ".jpeg":
					mimeType = "image/jpeg"
				case ".png":
					mimeType = "image/png"
				case ".svg":
					mimeType = "image/svg+xml"
				}
			}
			
			pageData.Media = append(pageData.Media, uploadedContent{
				FileType: fileType,
				Content:  filename,
				MimeType: mimeType,
			})
		}
	}
	
	// Create a map of media filenames for linking
	mediaMap := make(map[string]bool)
	for _, m := range pageData.Media {
		mediaMap[m.Content] = true
	}

	// Link text to media if content matches filename
	for i := range pageData.Texts {
		if mediaMap[pageData.Texts[i].Content] {
			pageData.Texts[i].LinkTarget = "/hub/assets/" + pageData.Texts[i].Content
		}
	}
	
	tpl.ExecuteTemplate(w, "index.html", pageData)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hub/", http.StatusSeeOther)
		return
	}

	// Limit upload size to 500MB
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)

	// It's a good practice to parse the form at the beginning.
	// This will handle both multipart and regular form data.
	// Set a max memory limit for parsing.
	if err := r.ParseMultipartForm(10 << 20); err != nil && err != http.ErrNotMultipart {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle text submission
	text := r.FormValue("text")
	if text != "" {
		// Append text directly to data.txt file
		file, err := os.OpenFile(dataFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			http.Error(w, "Unable to save text content.", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		
		if _, err := fmt.Fprintln(file, text); err != nil {
			http.Error(w, "Unable to save text content.", http.StatusInternalServerError)
			return
		}
	}

	// Handle file uploads
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		files := r.MultipartForm.File["file"]
		for _, handler := range files {
			// Make sure the file is not empty
			if handler.Size == 0 {
				continue
			}

			// Validate extension
			ext := strings.ToLower(filepath.Ext(handler.Filename))
			if !isValidExtension(ext) {
				continue
			}

			file, err := handler.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// Sanitize filename
			filename := filepath.Base(handler.Filename)

			f, err := os.OpenFile(filepath.Join(assetsDir, filename), os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				http.Error(w, "Unable to create the file for writing. Check your write access privilege.", http.StatusInternalServerError)
				return
			}
			defer f.Close()
			io.Copy(f, file)
		}
	}

	http.Redirect(w, r, "/hub/", http.StatusSeeOther)
}

func isValidExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".svg", ".mp4", ".webm", ".ogg", ".mov":
		return true
	}
	return false
}

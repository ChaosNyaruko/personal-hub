package main

import (
	"bufio"
	"crypto/subtle"
	"encoding/gob"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/sessions"
)

type uploadedContent struct {
	FileType string // "text", "image", or "video"
	Content  string
	MimeType string
}

var (
	tpl      *template.Template
	store    *sessions.CookieStore
	dataFile = "data.txt"
)

func init() {
	// WARNING: NOT so safe, it's just for my convinence!!
	store = sessions.NewCookieStore([]byte("secret-key"))
	store.Options = &sessions.Options{
		Secure: false, // make it work in LAN, when accessed by LAN addr.
		MaxAge: 7 * 24 * 3600,
	}
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("get wd error: %v", err)
	}
	log.Printf("cwd: %v", cwd)
	gob.Register([]uploadedContent{})
	tpl = template.Must(template.ParseGlob("templates/*.html"))

	hub := http.NewServeMux()

	hub.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
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
	var contentSlice []uploadedContent
	
	// Read text content from data.txt
	if file, err := os.Open(dataFile); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			contentSlice = append(contentSlice, uploadedContent{
				FileType: "text", 
				Content:  scanner.Text(), 
				MimeType: "",
			})
		}
	}
	
	// Read files from assets directory
	if files, err := os.ReadDir("assets"); err == nil {
		for _, file := range files {
			if !file.IsDir() {
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
				
				contentSlice = append(contentSlice, uploadedContent{
					FileType: fileType,
					Content:  filename,
					MimeType: mimeType,
				})
			}
		}
	}
	
	tpl.ExecuteTemplate(w, "index.html", contentSlice)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hub/", http.StatusSeeOther)
		return
	}

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
			file, err := handler.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer file.Close()

			f, err := os.OpenFile(filepath.Join("assets", handler.Filename), os.O_WRONLY|os.O_CREATE, 0666)
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

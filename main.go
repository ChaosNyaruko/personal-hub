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
	"sync"
	"time"

	"github.com/gorilla/sessions"
)

type uploadedContent struct {
	FileType string // "text", "image", or "video"
	Content  string
	MimeType string
}

var (
	mu       sync.Mutex
	data     []string
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

	loadData()

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

func loadData() {
	mu.Lock()
	defer mu.Unlock()
	file, err := os.Open(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return // No data file yet, which is fine.
		}
		panic(err)
	}
	defer file.Close()

	data = []string{} // Clear existing data
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data = append(data, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func saveData() {
	mu.Lock()
	defer mu.Unlock()
	file, err := os.Create(dataFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	for _, line := range data {
		fmt.Fprintln(file, line)
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
		session.Values["uploaded_content"] = []uploadedContent{}
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
	session.Values["uploaded_content"] = nil
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
	contentSlice, ok := session.Values["uploaded_content"].([]uploadedContent)
	if !ok {
		contentSlice = []uploadedContent{}
	}
	tpl.ExecuteTemplate(w, "index.html", contentSlice)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hub/", http.StatusSeeOther)
		return
	}

	session, err := store.Get(r, "session")
	if err != nil {
		http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
		return
	}
	contentSlice, ok := session.Values["uploaded_content"].([]uploadedContent)
	if !ok {
		contentSlice = []uploadedContent{}
	}

	// Handle text submission
	text := r.FormValue("text")
	if text != "" {
		mu.Lock()
		data = append(data, text)
		mu.Unlock()
		contentSlice = append(contentSlice, uploadedContent{FileType: "text", Content: text, MimeType: ""})
	}

	// Handle file upload
	file, handler, err := r.FormFile("file")
	if err == nil {
		defer file.Close()

		uploadSource := r.FormValue("upload_source")
		filename := handler.Filename
		ext := strings.ToLower(filepath.Ext(handler.Filename))

		if uploadSource == "paste" {
			filename = fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		}

		f, err := os.OpenFile(filepath.Join("assets", filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			http.Error(w, "Unable to create the file for writing. Check your write access privilege.", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		io.Copy(f, file)

		fileType := "image"
		mimeType := ""
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

		mu.Lock()
		data = append(data, filename)
		mu.Unlock()

		contentSlice = append(contentSlice, uploadedContent{FileType: fileType, Content: filename, MimeType: mimeType})
	} else if err != http.ErrMissingFile {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	saveData()
	session.Values["uploaded_content"] = contentSlice
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/hub/", http.StatusSeeOther)
}

package main

import (
	"bufio"
	"crypto/subtle"
	"encoding/gob"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	store    = sessions.NewCookieStore([]byte("secret-key"))
	dataFile = "data.txt"
)

func main() {
	gob.Register([]uploadedContent{})
	tpl = template.Must(template.ParseGlob("templates/*.html"))

	loadData()

	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/upload", authMiddleware(uploadHandler))

	fmt.Println("Server started at http://localhost:8080")
	http.ListenAndServe(":8080", nil)
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
			http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
			return
		}
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
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
			http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
			return
		}
		session.Values["authenticated"] = true
		session.Values["uploaded_content"] = []uploadedContent{}
		err = session.Save(r, w)
		if err != nil {
			http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tpl.ExecuteTemplate(w, "login.html", "Invalid credentials")
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session")
	if err != nil {
		http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
		return
	}
	session.Values["authenticated"] = false
	session.Values["uploaded_content"] = nil
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session")
	if err != nil {
		http.Error(w, "An error occurred, please try again later.", http.StatusInternalServerError)
		return
	}
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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
		http.Redirect(w, r, "/", http.StatusSeeOther)
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
		f, err := os.OpenFile(filepath.Join("assets", handler.Filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			http.Error(w, "Unable to create the file for writing. Check your write access privilege.", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		io.Copy(f, file)

		fileType := "image"
		mimeType := ""
		ext := strings.ToLower(filepath.Ext(handler.Filename))
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
		data = append(data, handler.Filename)
		mu.Unlock()

		contentSlice = append(contentSlice, uploadedContent{FileType: fileType, Content: handler.Filename, MimeType: mimeType})
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

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
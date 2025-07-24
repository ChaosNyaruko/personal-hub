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
	"sync"

	"github.com/gorilla/sessions"
)

type submittedContent struct {
	IsImage bool
	Content string
}

var (
	mu      sync.Mutex
	data    []string
	tpl     *template.Template
	store   = sessions.NewCookieStore([]byte("secret-key"))
	users   = map[string]string{"admin": "password"}
	dataFile = "data.txt"
)

func main() {
	gob.Register([]submittedContent{})
	tpl = template.Must(template.ParseGlob("templates/*.html"))

	loadData()

	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/submit", authMiddleware(submitHandler))

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
		session, _ := store.Get(r, "session")
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

	if pass, ok := users[username]; ok {
		if subtle.ConstantTimeCompare([]byte(password), []byte(pass)) == 1 {
			session, _ := store.Get(r, "session")
			session.Values["authenticated"] = true
			session.Values["submitted_content"] = []submittedContent{}
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	tpl.ExecuteTemplate(w, "login.html", "Invalid credentials")
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	session.Values["authenticated"] = false
	session.Values["submitted_content"] = nil
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	contentSlice, ok := session.Values["submitted_content"].([]submittedContent)
	if !ok {
		contentSlice = []submittedContent{}
	}
	tpl.ExecuteTemplate(w, "index.html", contentSlice)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	session, _ := store.Get(r, "session")
	contentSlice, ok := session.Values["submitted_content"].([]submittedContent)
	if !ok {
		contentSlice = []submittedContent{}
	}

	// Handle text submission
	text := r.FormValue("text")
	if text != "" {
		mu.Lock()
		data = append(data, text)
		mu.Unlock()
		saveData()
		contentSlice = append(contentSlice, submittedContent{IsImage: false, Content: text})
	}

	// Handle image upload
	file, handler, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		f, err := os.OpenFile(filepath.Join("assets", handler.Filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			http.Error(w, "Unable to create the file for writing. Check your write access privilege.", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		io.Copy(f, file)
		contentSlice = append(contentSlice, submittedContent{IsImage: true, Content: handler.Filename})
	} else if err != http.ErrMissingFile {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session.Values["submitted_content"] = contentSlice
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
package main

import (
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/pebble-dev/audio-debug-mode/adm"
	"log"
	"net/http"
	"os"
)

var store *sessions.CookieStore

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}
	store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	store.Options.Secure = true
	store.Options.HttpOnly = true
	store.Options.SameSite = http.SameSiteLaxMode
	a := adm.NewAdb(store)
	log.Println("Listening on :8000")
	log.Fatalln(a.ListenAndServe(":8000"))
}

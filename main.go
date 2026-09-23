package main

import (
	"fmt"
	"goCloudStorage/auth"
	"goCloudStorage/db"
	"goCloudStorage/handler"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	// Load local development credentials from .env.
	if err := godotenv.Load(); err != nil {
		log.Fatal("Failed to load .env: ", err)
	}

	if err := db.InitDB(); err != nil {
		log.Fatal(err)
	}

	defer db.DBConn().Close()

	registerRoutes(http.DefaultServeMux)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Failed to start server: %s", err.Error())
	}
}

func registerRoutes(mux *http.ServeMux) {
	// Public routes.
	mux.HandleFunc("GET /file/signup", handler.SignUpHandler)
	mux.HandleFunc("POST /file/signup", handler.SignUpHandler)
	mux.HandleFunc("POST /file/signin", handler.SigninHandler)

	// Protected routes.
	mux.HandleFunc("GET /file/home", auth.RequireAuth(handler.HomeHandler))
	mux.HandleFunc("GET /file/user/info", auth.RequireAuth(handler.UserInfoHandler))
	mux.HandleFunc("GET /file/upload", auth.RequireAuth(handler.UploadHandler))
	mux.HandleFunc("POST /file/upload", auth.RequireAuth(handler.UploadHandler))
	mux.HandleFunc("GET /file/meta", auth.RequireAuth(handler.GetFileMetaHandler))
	mux.HandleFunc("GET /file/download", auth.RequireAuth(handler.DownloadHandler))
	mux.HandleFunc("POST /file/update", auth.RequireAuth(handler.FileUpdateHandler))
	mux.HandleFunc("DELETE /file/delete", auth.RequireAuth(handler.FileDelHandler))
}

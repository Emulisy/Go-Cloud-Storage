package main

import (
	"fmt"
	"goCloudStorage/auth"
	"goCloudStorage/cache"
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

	if err := cache.InitRedis(); err != nil {
		log.Fatal(err)
	}
	defer cache.CloseRedis()

	registerRoutes(http.DefaultServeMux)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Failed to start server: %s", err.Error())
	}
}

func registerRoutes(mux *http.ServeMux) {
	// Public routes.
	mux.HandleFunc("GET /{$}", handler.LandingHandler)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("static/assets"))))
	mux.HandleFunc("GET /file/signin", handler.LoginPageHandler)
	mux.HandleFunc("GET /file/signup", handler.SignUpHandler)
	mux.HandleFunc("POST /api/users", handler.SignUpHandler)
	mux.HandleFunc("POST /api/sessions", handler.SigninHandler)
	mux.HandleFunc("POST /api/sessions/signout", handler.SignOutHandler)

	// Protected routes.
	mux.HandleFunc("GET /file/account", auth.RequirePageAuth(handler.AccountPageHandler))
	mux.HandleFunc("GET /file/home", auth.RequirePageAuth(handler.HomeHandler))
	mux.HandleFunc("GET /api/users/me", auth.RequireAuth(handler.UserInfoHandler))
	mux.HandleFunc("PATCH /api/users/me/name", auth.RequireAuth(handler.UpdateUserNameHandler))
	mux.HandleFunc("PATCH /api/users/me/password", auth.RequireAuth(handler.UpdateUserPwdHandler))
	mux.HandleFunc("PATCH /api/users/me/email", auth.RequireAuth(handler.UpdateUserEmailHandler))
	mux.HandleFunc("GET /file/upload", auth.RequirePageAuth(handler.UploadPageHandler))
	mux.HandleFunc("POST /api/files", auth.RequireAuth(handler.UploadHandler))
	mux.HandleFunc("GET /api/uploads/{filehash}", auth.RequireAuth(handler.UploadStatusHandler))
	mux.HandleFunc("POST /api/uploads", auth.RequireAuth(handler.InitialMPUploadHandler))
	mux.HandleFunc("PUT /api/uploads/{filehash}/parts/{index}", auth.RequireAuth(handler.UploadPartHandler))
	mux.HandleFunc("POST /api/uploads/{filehash}/completion", auth.RequireAuth(handler.UploadCompleteHandler))
	mux.HandleFunc("GET /api/files", auth.RequireAuth(handler.GetFileMetaHandler))
	mux.HandleFunc("GET /api/files/{id}/content", auth.RequireAuth(handler.DownloadHandler))
	mux.HandleFunc("PATCH /api/files/{id}", auth.RequireAuth(handler.FileUpdateHandler))
	mux.HandleFunc("DELETE /api/files/{id}", auth.RequireAuth(handler.FileDelHandler))
}

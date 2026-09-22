package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"goCloudStorage/db"
	"goCloudStorage/handler"
	"log"
	"net/http"
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

	http.HandleFunc("/file/upload", handler.UploadHandler)
	http.HandleFunc("/file/upload/suc", handler.UploadSucHandler)
	http.HandleFunc("/file/meta", handler.GetFileMetaHnadler)
	http.HandleFunc("/file/download", handler.DownloadHandler)
	http.HandleFunc("/file/update", handler.FileUpdateHandler)
	http.HandleFunc("/file/delete", handler.FileDelHandler)
	http.HandleFunc("/file/signup", handler.SignUpHandler)
	http.HandleFunc("/file/signin", handler.SigninHandler)
	http.HandleFunc("GET /file/home", handler.HomeHandler)
	http.HandleFunc("GET /file/user/info", handler.UserInfoHandler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Failed to start server: %s", err.Error())
	}
}

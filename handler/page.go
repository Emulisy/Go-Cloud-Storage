package handler

import (
	"net/http"

	"goCloudStorage/auth"
)

func LandingHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/view/landing.html")
}

func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, "static/view/login.html")
}

func AccountPageHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, "static/view/account.html")
}

func UploadPageHandler(
    w http.ResponseWriter,
    r *http.Request,
    user auth.User,
) {
    http.ServeFile(
        w,
        r,
        "./static/view/index.html",
    )
}
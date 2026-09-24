package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goCloudStorage/auth"
)

func TestAPIRoutesRequireAuthentication(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux)
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/users/me"},
		{"PATCH", "/api/users/me/name"},
		{"PATCH", "/api/users/me/password"},
		{"PATCH", "/api/users/me/email"},
		{"GET", "/api/files"},
		{"POST", "/api/files"},
		{"PATCH", "/api/files/1"},
		{"DELETE", "/api/files/1"},
		{"GET", "/api/files/1/content"},
		{"POST", "/api/uploads"},
		{"GET", "/api/uploads/hash"},
		{"PUT", "/api/uploads/hash/parts/0"},
		{"POST", "/api/uploads/hash/completion"},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
		if w.Code != http.StatusUnauthorized || w.Header().Get("Location") != "" {
			t.Errorf("%s %s: expected 401 without redirect, got %d", route.method, route.path, w.Code)
		}
	}
}

func TestRetiredRoutesAndWrongMethods(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux)
	for _, route := range []struct {
		method, path string
		status       int
	}{
		{"POST", "/file/signup", 405},
		{"POST", "/file/signin", 404},
		{"GET", "/file/user/info", 404},
		{"POST", "/file/user/name", 404},
		{"POST", "/file/user/password", 404},
		{"POST", "/file/user/email", 404},
		{"POST", "/file/upload", 405},
		{"POST", "/file/upload/init", 404},
		{"POST", "/file/upload/part", 404},
		{"POST", "/file/upload/complete", 404},
		{"GET", "/file/meta", 404},
		{"GET", "/file/download", 404},
		{"POST", "/file/update", 404},
		{"DELETE", "/file/delete", 404},
		{"GET", "/api/users", 405},
		{"GET", "/api/sessions", 405},
		{"POST", "/api/users/me/name", 405},
		{"POST", "/api/files/1", 405},
		{"POST", "/api/uploads/hash/parts/0", 405},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
		if w.Code != route.status {
			t.Errorf("%s %s: got %d, want %d", route.method, route.path, w.Code, route.status)
		}
		if route.status == 405 && w.Header().Get("Allow") == "" {
			t.Errorf("%s: missing Allow header", route.path)
		}
	}
}

func TestPathValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	token, err := auth.GenerateToken(1)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerRoutes(mux)
	hash := strings.Repeat("a", 64)
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/files/bad/content?id=1", ""},
		{"DELETE", "/api/files/0?id=1", ""},
		{"PATCH", "/api/files/bad?id=1", "id=1&name=renamed"},
		{"GET", "/api/uploads/bad?filehash=" + hash, ""},
		{"PUT", "/api/uploads/bad/parts/0?filehash=" + hash, ""},
		{"PUT", "/api/uploads/" + hash + "/parts/bad?index=0", ""},
		{"PUT", "/api/uploads/" + hash + "/parts/-1", ""},
		{"POST", "/api/uploads/bad/completion?filehash=" + hash, ""},
		{"POST", "/api/uploads/" + strings.Repeat("z", 64) + "/completion", ""},
		{"PATCH", "/api/users/me/name", "userName=ab"},
		{"PATCH", "/api/users/me/password", "currentPwd=valid-password&newPwd=x"},
		{"PATCH", "/api/users/me/email", "email=invalid"},
	} {
		r := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Errorf("%s %s: got %d, want 400", route.method, route.path, w.Code)
		}
	}
}

func TestPageRoutesRemain(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	token, err := auth.GenerateToken(1)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerRoutes(mux)
	for _, path := range []string{"/file/signup", "/file/home", "/file/upload"} {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(strings.ToLower(w.Body.String()), "<!doctype html>") {
			t.Errorf("%s: expected HTML page, got %d", path, w.Code)
		}
	}
	for _, path := range []string{"/file/home", "/file/upload"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 303 || w.Header().Get("Location") != "/file/signup" {
			t.Errorf("%s: page authentication redirect changed", path)
		}
	}
}

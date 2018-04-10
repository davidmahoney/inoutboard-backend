package main

import (
	"net/http"
)

func AddHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

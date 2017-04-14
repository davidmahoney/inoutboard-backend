package main

import (
	//"github.com/goji/httpauth"
	"fmt"
	"net/http"
)

func ldapAuthFunc(user, pass string, r *http.Request) bool {
	fmt.Printf("User %s logged in\n", user)
	return true
}

func unauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4050")
	w.Header().Add("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, HEAD")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	if r.Method == "OPTIONS" {
		return
	} else {
		http.Error(w, "", http.StatusUnauthorized)
	}
}

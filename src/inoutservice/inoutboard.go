package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Status int

const (
	In = iota
	Out
	InField
)

type Person struct {
	Name     string
	Username string
	Status   Status
	Remarks  string
}

func loadUser(username string) *Person {
	fmt.Println("Request for user ", username)
	user := Person{
		Name:     "David",
		Username: "dmahoney",
		Status:   In,
		Remarks:  "",
	}
	return &user
}

func loadUsers() []Person {
	people := GetUsers()
	return people
}

func handler(w http.ResponseWriter, r *http.Request) {
	var user = loadUser(r.URL.Path[len("/user/"):])
	json.NewEncoder(w).Encode(user)
}

func main() {
	createDb()
	http.HandleFunc("/user/", handler)
	http.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		var users = loadUsers()
		json.NewEncoder(w).Encode(users)
	})
	http.ListenAndServe(":8080", nil)
}

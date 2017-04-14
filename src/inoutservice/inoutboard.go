package main

import (
	"encoding/json"
	"fmt"
	"github.com/goji/httpauth"
	"net/http"
)

type Status int

const (
	In = iota
	Out
	InField
)

type Person struct {
	ID          int    `jsonapi:"primary,people"`
	Name        string `jsonapi:"attr,name"`
	Username    string `jsonapi:"attr,username"`
	Status      Status `jsonapi:"attr,status"`
	StatusValue string `jsonapi:"attr,statusvalue"`
	Remarks     string `jsonapi:"attr,notes"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4050")
	w.Header().Add("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, HEAD")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	switch r.Method {
	case "GET":
		var user *Person
		var err error
		username, _, _ := r.BasicAuth()
		if r.URL.Path[len("/user/"):] != "" {
			user, err = GetPerson(r.URL.Path[len("/user/"):])
		} else {
			user, err = GetPerson(username)
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err = json.NewEncoder(w).Encode(user); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	case "PUT": // should probably actually return something
		fmt.Printf("Saving user...\n")
		person := new(Person)
		err := json.NewDecoder(r.Body).Decode(person)

		username, _, _ := r.BasicAuth()
		if username != person.Username {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			fmt.Printf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = SetPerson(person)
		return
	case "OPTIONS":
		return

	}
}

func peopleHandler(w http.ResponseWriter, r *http.Request) {
	user, _, _ := r.BasicAuth()
	fmt.Printf("User is %s\n", user)
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4050")
	w.Header().Add("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, HEAD")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	if r.Method == "OPTIONS" {
		return
	}

	peopleInterface := make([]interface{}, 0)
	people, err := GetUsers()
	if err != nil {
	}
	for _, person := range people {
		fmt.Printf("Person: %s\n", person.Name)
		peopleInterface = append(peopleInterface, person)
	}
	fmt.Printf("GetUsers returned %d people\n", len(peopleInterface))
	//if err := jsonapi.MarshalManyPayload(w, peopleInterface); err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//}
	if err := json.NewEncoder(w).Encode(peopleInterface); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func main() {
	createDb()
	authOpts := httpauth.AuthOptions{
		Realm:               "rdffg",
		AuthFunc:            ldapAuthFunc,
		UnauthorizedHandler: http.HandlerFunc(unauthorizedHandler),
	}
	http.Handle("/user/", httpauth.BasicAuth(authOpts)(http.HandlerFunc(handler)))
	http.Handle("/people/", httpauth.BasicAuth(authOpts)(http.HandlerFunc(peopleHandler)))
	http.Handle("/people", httpauth.BasicAuth(authOpts)(http.HandlerFunc(peopleHandler)))
	http.ListenAndServe(":8080", nil)
}

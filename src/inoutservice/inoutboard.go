package main

import (
	//"encoding/json"
	"fmt"
	"github.com/google/jsonapi"
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
	switch r.Method {
	case "GET":
		user, err := GetPerson(r.URL.Path[len("/user/"):])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err = jsonapi.MarshalOnePayload(w, user); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	case "PUT": // should probably actually return something
		person := new(Person)
		person := jsonapi.UnmarshalPayload(r.Body, person)
		err := SetPerson(person)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
}

func peopleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://zaphod:4200")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	peopleInterface := make([]interface{}, 0)
	people, err := GetUsers()
	if err != nil {
	}
	for _, person := range people {
		fmt.Printf("Person: %s\n", person.Name)
		peopleInterface = append(peopleInterface, person)
	}
	fmt.Printf("GetUsers returned %d people\n", len(peopleInterface))
	if err := jsonapi.MarshalManyPayload(w, peopleInterface); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func main() {
	createDb()
	http.HandleFunc("/user/", handler)
	http.HandleFunc("/people/", peopleHandler)
	http.HandleFunc("/people", peopleHandler)
	http.ListenAndServe(":8080", nil)
}

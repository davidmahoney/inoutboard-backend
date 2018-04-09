package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/gcfg.v1"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type Status int

const (
	In = iota
	Out
	InField
)

type Person struct {
	ID              int
	Name            string
	Username        string
	ReturnTime      string
	RegularSchedule string
	Department      string
	Status          Status
	StatusValue     string
	Remarks         string
	Mobile          string
	Telephone       string
	Office          string
	LastEditor      string
	LastEditTime    time.Time
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4050")
	w.Header().Add("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, HEAD")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
	username := usernameFromContext(r.Context())

	switch r.Method {
	case "GET":
		var user *Person
		var err error
		log.Printf(r.URL.Path)
		if r.URL.Path[len("user/"):] != "" {
			user, err = GetPerson(r.URL.Path[len("user/"):])
		} else {
			log.Printf("user from context: %s", username)
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
	case "PUT":
		fmt.Printf("Saving user...\n")
		person := new(Person)
		err := json.NewDecoder(r.Body).Decode(person)

		if err != nil {
			fmt.Printf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = SetPerson(person, username)
		updated, err := GetPerson(username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			if err = json.NewEncoder(w).Encode(updated); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
		return
	case "OPTIONS":
		return

	}
}

func peopleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4050")
	w.Header().Add("Access-Control-Allow-Methods", "GET, OPTIONS, HEAD")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
	switch r.Method {
	case "OPTIONS":
		break

	case "GET":
		peopleInterface := make([]interface{}, 0)
		people, err := GetUsers()
		if err != nil {
		}
		for _, person := range people {
			fmt.Printf("Person: %s\n", person.Name)
			peopleInterface = append(peopleInterface, person)
		}
		fmt.Printf("GetUsers returned %d people\n", len(peopleInterface))

		if err := json.NewEncoder(w).Encode(peopleInterface); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
	return
}

func main() {
	createDb()

	port := 8888
	var cfg Config
	err := gcfg.ReadFileInto(&cfg, "config.ini")
	if err != nil {
		log.Printf(err.Error())
		panic("could not open config.ini")
	}

	log.Printf("config ldapServer: %s", cfg.Auth.LdapServer)

	if cfg.Net.Port > 0 {
		port = cfg.Net.Port
	}

	authOptions := AuthorizationOptions{
		realm:          cfg.Auth.Realm,
		ldapServer:     cfg.Auth.LdapServer,
		port:           cfg.Auth.LdapPort,
		username:       cfg.Auth.Username,
		password:       cfg.Auth.BindPassword,
		ldapSearchBase: cfg.Auth.LdapSearchBase,
	}

	var verbose bool
	var update bool

	if len(os.Args[1:]) > 0 { // found command-line args
		for _, p := range os.Args[1:] {
			switch p {
			case "--update-users":
				update = true

			case "--verbose":
				verbose = true
			}
		}
		if update { // run ldap update
			if !verbose {
				log.SetOutput(ioutil.Discard)
			}
			UpdateLdap(authOptions)
			return
		}
	}

	http.Handle("/api/user/", AuthorizationMiddleware(authOptions, AddHeaders(http.StripPrefix("/api/", http.HandlerFunc(handler)))))
	http.Handle("/api/people/", AuthorizationMiddleware(authOptions, AddHeaders(http.HandlerFunc(peopleHandler))))
	http.Handle("/api/people", AuthorizationMiddleware(authOptions, AddHeaders(http.HandlerFunc(peopleHandler))))
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)
	log.Printf("Starting service on port %d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

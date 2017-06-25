package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/leonelquinteros/gorand"
	"gopkg.in/ldap.v2"
	"log"
	"net/http"
	_ "strings"
	"time"
)

var authOptions *AuthorizationOptions

type Error struct {
	message string
	path    string
}

type AuthorizationOptions struct {
	realm          string
	ldapServer     string
	port           int
	username       string
	password       string
	ldapSearchBase string
}

type Credentials struct {
	Username string
	Password string
}

// setup handlers
func init() {
	var logoutHandler = http.HandlerFunc(Logout)
	var loginHandler = http.HandlerFunc(Login)
	http.Handle("/login", loginHandler)
	http.Handle("/logout", logoutHandler)
}

// LdapAuthFunc authenticates a user against an LDAP server
// The Request parameter is probably not necessary.
func LdapAuthFunc(creds *Credentials) bool {
	hostaddr := fmt.Sprintf("%s:%d", authOptions.ldapServer, authOptions.port) // move to config file
	conn, err := ldap.Dial("tcp", hostaddr)
	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatal(err)
	}

	err = conn.Bind(authOptions.realm+"\\"+creds.Username, creds.Password)
	if err == nil {
		return true
	} else {
		log.Printf("LDAP: %s", err.Error())
		return false
	}
}

func CreateUser(username string) error {
	hostaddr := fmt.Sprintf("%s:%d", authOptions.ldapServer, authOptions.port)
	conn, err := ldap.Dial("tcp", hostaddr)
	if err != nil {
		log.Fatal(err)
		return err
	}

	defer conn.Close()

	err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatal(err)
		return err
	}

	err = conn.Bind(authOptions.realm+"\\"+authOptions.username, authOptions.password)
	if err != nil {
		log.Fatal(err)
		return err
	}

	searchRequest := ldap.NewSearchRequest(
		authOptions.ldapSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(sAMAccountName=%s))", username),
		[]string{"dn", "cn", "title", "department", "telephoneNumber"},
		nil,
	)
	res, err := conn.Search(searchRequest)
	if err != nil || len(res.Entries) != 1 {
		log.Fatal(err)
	}

	ldapPerson := res.Entries[0]

	err = AddPerson(
		username,
		ldapPerson.GetAttributeValue("cn"),
		ldapPerson.GetAttributeValue("department"),
		ldapPerson.GetAttributeValue("telephoneNumber"),
	)
	return err
}

func AuthorizationMiddleware(options AuthorizationOptions, next http.Handler) http.Handler {
	authOptions = &options
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var session string = ""
		cookies := r.Cookies()
		for _, cookie := range cookies {
			if cookie.Name == "session" {
				session = cookie.Value
				log.Println("Session ", cookie.Value)
				break
			}
		}

		if username, err := ValidateSession(session); err == nil {
			next.ServeHTTP(w, r.WithContext(newContextWithUsername(r.Context(), username)))
		} else {
			log.Println("no session found")
			sessionErr := Error{message: "unauthorized", path: "/login"}
			if err := json.NewEncoder(w).Encode(sessionErr); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	log.Println("logged out")
	var cookie http.Cookie
	cookies := r.Cookies()
	for _, c := range cookies {
		if c.Name == "session" {
			cookie = *c
			break
		}
	}
	if err := RemoveSession(cookie.Value); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}
	cookie.Value = ""
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, &cookie)
	w.Write([]byte("Logged out"))
}

func Login(w http.ResponseWriter, r *http.Request) {
	log.Println("Creating session")
	session, err := gorand.UUID()
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}

	creds := new(Credentials)
	err = json.NewDecoder(r.Body).Decode(creds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if LdapAuthFunc(creds) {
		var person *Person

		if person, err = GetPerson(creds.Username); err != nil {
			// create user from ldap store
			if err = CreateUser(creds.Username); err != nil {
				log.Fatalf("Failed to create user: %s", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			log.Printf("Created user")
			if person, err = GetPerson(creds.Username); err != nil {
				log.Fatal(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
		if err = CreateSession(session, person.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("created session %s", session)

		cookie := &http.Cookie{Name: "session", Value: session, HttpOnly: false}
		http.SetCookie(w, cookie)
		w.Write([]byte("success"))

	} else {
		log.Printf("login failed")
		sessionErr := Error{message: "login failed", path: "/login"}
		if err := json.NewEncoder(w).Encode(sessionErr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type requestKey int

const requestUsernameKey = 0

func usernameFromContext(ctx context.Context) string {
	return ctx.Value(requestUsernameKey).(string)
}

func newContextWithUsername(ctx context.Context, username string) context.Context {
	u := context.WithValue(ctx, requestUsernameKey, username).Value(requestUsernameKey).(string)
	return context.WithValue(ctx, requestUsernameKey, username)
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

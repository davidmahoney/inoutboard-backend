package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/leonelquinteros/gorand"
	"gopkg.in/ldap.v2"
	"net/http"
	"strings"
	_ "strings"
	"time"
)

var authOptions *AuthorizationOptions

// An error that can be serialized to
// JSON and returned to the client
type Error struct {
	// Error message
	Message string
	// The http path to the api endpoint
	// to fix the error, eg: /login
	Path string
}

// A set of options necessary to find
// and login to a LDAP server
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
		log.Printf("Could not connect to LDAP: %s", err)
		log.Fatal(err)
	}

	dn, err := SanitizeDN(creds.Username)
	if err != nil {
		log.Infof("User %s attempted authentication with an invalid username", creds.Username)
		return false
	}

	if len(creds.Password) == 0 {
		return false
	}

	if strings.LastIndexAny(dn, "@") < 0 { // not an email address
		dn = authOptions.realm + "\\" + dn
	}
	err = conn.Bind(dn, creds.Password)
	if err == nil {
		log.Debugf("User %s logged in", creds.Username)
		return true
	} else {
		log.Printf("LDAP: %s", err.Error())
		return false
	}
}

// Escape a string for use in a DN
// This follows the rules for allowed characters
// in a samaccountname field in Active Directory
// Other LDAP implementations may differ
func SanitizeDN(dn string) (string, error) {
	var newdn string = dn
	forbiddenChars := "\"[]:;|=+*?<>/\\,"

	// no empty distinguished names allowed
	if len(dn) == 0 {
		return "", fmt.Errorf("forbidden")
	}

	if strings.LastIndexAny(dn, forbiddenChars) >= 0 {
		return "", fmt.Errorf("forbidden")
	}
	// characters with values < 32 are also forbidden
	for _, c := range dn {
		if int(c) < 32 {
			return "", fmt.Errorf("forbidden")
		}
	}
	// escape a leading space or # character
	if len(dn) > 0 && (dn[0] == ' ' || dn[0] == '#') {
		newdn = "\\" + dn
	}

	// escape \ * ( ) and NUL characters
	replacer := strings.NewReplacer("\\", "\\5c", "*", "\\2a", "(", "\\28", ")", "\\29", "\u0000", "\\00")
	newdn = replacer.Replace(newdn)
	// escape a trailing space
	if dn[len(dn)-1] == ' ' {
		newdn = strings.TrimSpace(newdn)
		newdn += "\\ "
	}
	log.Debugf("Replaced username \"%s\" with \"%s\"", dn, newdn)
	return newdn, nil
}

// Find a person in LDAP
func FindUser(username string) (Person, error) {
	var user Person
	if authOptions == nil {
		log.Panicf("Auth options should not be nil")
	}
	hostaddr := fmt.Sprintf("%s:%d", authOptions.ldapServer, authOptions.port)
	conn, err := ldap.Dial("tcp", hostaddr)
	if err != nil {
		log.Fatal(err)
		return user, err
	}

	defer conn.Close()

	err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Fatal(err)
		return user, err
	}

	err = conn.Bind(authOptions.realm+"\\"+authOptions.username, authOptions.password)
	if err != nil {
		log.Fatal(err)
		return user, err
	}

	dn, err := SanitizeDN(username)
	if err != nil {
		return user, fmt.Errorf("bad username")
	}

	var queryString string
	if strings.LastIndexAny(dn, "@") > 0 {
		queryString = fmt.Sprintf("(&(objectClass=organizationalPerson)(userPrincipalName=%s))", dn)
	} else {
		queryString = fmt.Sprintf("(&(objectClass=organizationalPerson)(sAMAccountName=%s))", dn)
	}

	searchRequest := ldap.NewSearchRequest(
		authOptions.ldapSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		queryString,
		[]string{"userPrincipalName", "cn", "title", "department", "telephoneNumber", "mobile", "physicalDeliveryOfficeName"},
		nil,
	)
	res, err := conn.Search(searchRequest)
	if err != nil || len(res.Entries) != 1 {
		log.Fatal(err)
	}

	ldapPerson := res.Entries[0]

	user = Person{
		Username:   ldapPerson.GetAttributeValue("userPrincipalName"),
		Name:       ldapPerson.GetAttributeValue("cn"),
		Department: ldapPerson.GetAttributeValue("department"),
		Telephone:  ldapPerson.GetAttributeValue("telephoneNumber"),
		Mobile:     ldapPerson.GetAttributeValue("mobile"),
		Office:     ldapPerson.GetAttributeValue("physicalDeliveryOfficeName"),
		Title:      ldapPerson.GetAttributeValue("title"),
	}
	return user, err
}

// Create a user for a given username. This user must exist
// in LDAP
func CreateUser(username string) (*Person, error) {
	var err error
	var user *Person = new(Person)
	*user, err = FindUser(username)
	if err != nil {
		return nil, err
	}

	if sqlUser, err := GetPerson(user.Username); sqlUser != nil {
		return sqlUser, err
	}

	user, err = AddPerson(
		user.Username,
		user.Name,
		user.Department,
		user.Telephone,
		user.Mobile,
		user.Office,
		user.Title,
	)
	return user, err
}

// Update all the database users with attributes
// from LDAP. Accordingly, this takes a set of
// LDAP connection options as a parameter
func UpdateLdap(options AuthorizationOptions) error {
	authOptions = &options
	// get users
	people, err := GetUsers()
	if err != nil {
		log.Fatalf("Failed to get users from the database: %s", err.Error())
		return err
	}
	// for each user, get the LDAP entry
	for _, user := range people {
		updated, err := FindUser(user.Username)
		if err != nil {
			log.Printf("Failed to get user %s from the LDAP Server: %s", user.Username, err.Error())
		}
		// and update the database
		user.Name = updated.Name
		user.Department = updated.Department
		user.Office = updated.Office
		user.Telephone = updated.Telephone
		user.Mobile = updated.Mobile
		user.Title = updated.Title
		log.Debugf("updating %s with name = %s, department = %s, office = %s telephone = %s, mobile = %s, title = %s",
			user.Username,
			user.Name,
			user.Department,
			user.Office,
			user.Telephone,
			user.Mobile,
			user.Title)
		if SetPersonDetails(user) != nil {
			log.Printf("Failed to update user %s: %s", user.Username, err)
		}

		fmt.Printf(". ")
	}
	fmt.Printf("\n")
	return nil
}

// Handles cookie-based authentication. An incoming
// request will have its session ID read from a cookie, and if
// the session is not valid, returns a JSON-encoded response
// redirecting to the Login api endpoint.
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
			sessionErr := &Error{Message: "unauthorized", Path: "/login"}
			log.Printf(sessionErr.Message)
			content, _ := json.Marshal(sessionErr)
			http.Error(w, string(content), http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(sessionErr); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
}

// Removes a session from the database and sets
// the cookie to be expired.
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

// Authenticate a user and create a session in the
// database, returning a cookie with the session ID
func Login(w http.ResponseWriter, r *http.Request) {
	log.Println("Creating session")
	id, err := gorand.UUIDv4()
	session, err := gorand.MarshalUUID(id)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
	}

	creds := new(Credentials)
	err = json.NewDecoder(r.Body).Decode(creds)
	creds.Username = strings.TrimSpace(creds.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if LdapAuthFunc(creds) {
		var person *Person

		if person, err = GetPerson(creds.Username); err != nil {
			// create user from ldap store
			person, err = CreateUser(creds.Username)
			if err != nil {
				log.Fatalf("Failed to create user: %s", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
		if err = CreateSession(session, person.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("created session %s", session)

		cookie := &http.Cookie{
			Name:     "session",
			Value:    session,
			HttpOnly: false,
			Expires:  time.Now().AddDate(1, 0, 0),
		}
		http.SetCookie(w, cookie)
		w.Write([]byte("success"))

	} else {
		log.Printf("login failed")
		sessionErr := Error{Message: "login failed", Path: "/login"}
		content, err := json.Marshal(sessionErr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, string(content), http.StatusUnauthorized)
	}
}

const requestUsernameKey = 0

// get the username from a context object
func usernameFromContext(ctx context.Context) string {
	return ctx.Value(requestUsernameKey).(string)
}

// return a new context object with the username in it
func newContextWithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, requestUsernameKey, username)
}

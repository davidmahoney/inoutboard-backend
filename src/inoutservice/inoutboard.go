package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/bakins/logrus-middleware"
	"github.com/coreos/go-systemd/daemon"
	"gopkg.in/gcfg.v1"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// A status code for a person
// e.g. In, Out, etc.
type Status int

// The actual status codes These *should*
// mirror codes held in the status table in the
// database. If they don't, bad things probably
// won't happen.
const (
	In = iota
	Out
	InField
)

// A person record
type Person struct {
	ID           int
	Name         string
	Username     string
	Department   string
	Status       Status
	StatusValue  string
	Remarks      string
	Mobile       string
	Telephone    string
	Office       string
	Title        string
	LastEditor   string
	LastEditTime time.Time
}

// Get or set an individual user
func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Methods", "GET, PUT, OPTIONS, HEAD")
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
		log.Debugf("Saving user...")
		person := new(Person)
		err := json.NewDecoder(r.Body).Decode(person)

		if err != nil {
			log.Error(err.Error())
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

// get a list of people from the database
func peopleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Methods", "GET, OPTIONS, HEAD")
	switch r.Method {
	case "OPTIONS":
		break

	case "GET":
		peopleInterface := make([]interface{}, 0)
		people, err := GetUsers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, person := range people {
			log.Debugf("Person: %s", person.Name)
			peopleInterface = append(peopleInterface, person)
		}
		log.Debugf("GetUsers returned %d people", len(peopleInterface))

		if err := json.NewEncoder(w).Encode(peopleInterface); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
	return
}

// get any applicable environment variables (if they're set)
func getEnvArgs() (string, *string) {
	var staticFiles *string
	config := os.Getenv("INOUTBOARD_CONFIG") // use whatever we find in the env
	if config == "" {                        // or use the file in /etc if it exists
		if _, err := os.Stat("/etc/inoutboard.d/config.ini"); os.IsNotExist(err) {
			config = "config.ini"
		} else {
			config = "/etc/inoutboard.d/config.ini"
		}
	}
	_, set := os.LookupEnv("INOUTBOARD_STATIC")
	if !set {
		staticFiles = nil
	} else {
		st := os.Getenv("INOUTBOARD_STATIC")
		staticFiles = &st
	}
	return config, staticFiles
}

func init() {
	// set up logging
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})
	switch os.Getenv("INOUTBOARD_DEBUG") {
	case "debug":
		{
			log.SetLevel(log.DebugLevel)
			log.SetFormatter(&log.TextFormatter{})
		}
	case "info":
		{
			log.SetLevel(log.InfoLevel)
		}
	case "warn":
		{
			log.SetLevel(log.WarnLevel)
		}
	case "error":
		{
			log.SetLevel(log.ErrorLevel)
		}
	default:
		{
			log.SetLevel(log.WarnLevel)
		}
	}
}

// Main function runs at application start
func main() {
	// get evnironment variables
	configPath, staticFilesPath := getEnvArgs()

	port := 8888

	// read the config file
	log.Printf("Using configuration file %s", configPath)
	var cfg Config
	err := gcfg.ReadFileInto(&cfg, configPath)
	if err != nil {
		log.Printf(err.Error())
		panic("could not open config.ini")
	}

	if cfg.Files.StaticFilesPath == "" {
		if staticFilesPath != nil {
			cfg.Files.StaticFilesPath = *staticFilesPath
		} else {
			cfg.Files.StaticFilesPath = "static"
		}
	}
	log.Printf("Static files: %s", cfg.Files.StaticFilesPath)

	log.Printf("config ldapServer: %s", cfg.Auth.LdapServer)
	if cfg.Files.DbPath != "" {
		log.Printf("using db: %s", cfg.Files.DbPath)
		createDb(cfg.Files.DbPath)
	} else {
		log.Fatal("DbPath must be defined in the [Files] section of the config file.")
	}

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

	// parse command line args
	// if the --update-users argument is found
	// update the users from LDAP and exit
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

	// configure the server
	logger := log.New()
	logger.SetLevel(log.StandardLogger().Level)
	logger.Out = log.StandardLogger().Out
	logger.Formatter = log.StandardLogger().Formatter

	l := logrusmiddleware.Middleware{
		Name:   "inoutboard",
		Logger: logger,
	}

	http.Handle("/api/user/", l.Handler(AuthorizationMiddleware(authOptions, AddHeaders(http.StripPrefix("/api/", http.HandlerFunc(handler)))), "user"))
	http.Handle("/api/people/", l.Handler(AuthorizationMiddleware(authOptions, AddHeaders(http.HandlerFunc(peopleHandler))), "people"))
	//http.Handle("/api/people", l.Handler(AuthorizationMiddleware(authOptions, AddHeaders(http.HandlerFunc(peopleHandler))), "people"))
	fs := http.FileServer(http.Dir(cfg.Files.StaticFilesPath))
	http.Handle("/", fs)
	log.Printf("Starting service on port %d", port)
	socket, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	// configure for systemd
	daemon.SdNotify(false, "READY=1")
	go func() {
		interval, err := daemon.SdWatchdogEnabled(false)
		if err != nil || interval == 0 {
			return
		}
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		for {

			client := &http.Client{Transport: tr, Timeout: time.Second * 2}
			res, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/", port))
			defer res.Body.Close()

			if err == nil {
				daemon.SdNotify(false, "WATCHDOG=1")
			}
			time.Sleep(interval / 3)
		}
	}()

	// Finally start serving clients
	err = http.ServeTLS(socket, nil, filepath.Join(filepath.Dir(configPath), "server.crt"),
		filepath.Join(filepath.Dir(configPath), "server.key"))
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

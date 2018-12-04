package main

import (
	"database/sql"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	_ "github.com/mattn/go-sqlite3"
	"sync"
)

var conn *sql.DB

var statusCodes map[int]Status
var mutex *sync.Mutex

func init() {
	mutex = &sync.Mutex{}
}

func ValidateSession(sessionID string) (string, error) {
	stmt, err := conn.Prepare("SELECT username FROM sessions JOIN people ON (person_id = people.id) WHERE sessions.id = ?")
	defer stmt.Close()
	checkErr(err)
	res, err := stmt.Query(sessionID)
	defer res.Close()
	checkErr(err)
	var username string
	rows := 0
	for res.Next() {
		if err = res.Scan(&username); err != nil {
			return "", err
		}
		rows++
	}
	if rows == 1 {
		return username, nil
	} else {
		return "", errors.New("session not found")
	}
}

func CreateSession(sessionID string, userID int) error {
	stmt, err := conn.Prepare("INSERT INTO sessions (id, person_id) VALUES (?, ?)")
	defer stmt.Close()
	checkErr(err)
	stmt.Exec(sessionID, userID)
	checkErr(err)
	return err
}

func RemoveSession(sessionID string) error {
	stmt, err := conn.Prepare("DELETE FROM sessions WHERE id = ?")
	defer stmt.Close()
	checkErr(err)
	_, err = stmt.Exec(sessionID)
	checkErr(err)
	return err
}

func checkErr(err error) {
	if err != nil {
		log.Error(err.Error())
		log.Error(" Noooooo!\n")
	}
}

func AddPerson(username string, name string, department string, telephone string, mobile string, office string, title string) (*Person, error) {
	stmt, err := conn.Prepare("INSERT INTO people (username, name, status, department, mobile, telephone, office, title) VALUES (?,?,?,?,?,?,?,?)")
	defer stmt.Close()
	if err != nil {
		log.Fatal(err)
	}

	// there has to be a status code 0 in the db or this will fail
	_, err = stmt.Exec(username, name, 0, department, mobile, telephone, office, title)
	if err != nil {
		return nil, err
	}

	person, err := GetPerson(username)
	log.Infof("Added %s to the db", username)
	return person, err
}

func createDb(dbPath string) {
	db, err := sql.Open("sqlite3", dbPath)
	checkErr(err)
	if err != nil {
		log.Fatal("Could not open the database file")
	}
	conn = db

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table'")
	defer rows.Close()
	tables := make(map[string]string)
	var table string
	checkErr(err)
	for rows.Next() {
		err = rows.Scan(&table)
		checkErr(err)
		log.Infof("found table %s", table)
		tables[table] = table
	}

	if _, ok := tables["status"]; !ok {
		log.Print("creating status table")
		res, err := db.Exec("CREATE TABLE status (id INTEGER PRIMARY KEY, value TEXT)")
		checkErr(err)
		log.Print("insert status values")
		stmt, err := db.Prepare("INSERT INTO status (value) VALUES (?)")
		checkErr(err)
		res, err = stmt.Exec("In")
		rows, err := res.RowsAffected()
		if rows == 0 {
			panic(err)
		}
		checkErr(err)
		res, err = stmt.Exec("Out")
		checkErr(err)
		res, err = stmt.Exec("In Field")
		checkErr(err)
	}
	log.Print("creating people table")
	if _, ok := tables["people"]; !ok {
		_, err = db.Exec("CREATE TABLE people (id INTEGER PRIMARY KEY, username TEXT UNIQUE, name TEXT NOT NULL, department TEXT null, mobile TEXT not null default '', telephone TEXT not null default '', office TEXT not null default '', title TEXT not null default '', status int REFERENCES status(id), notes TEXT DEFAULT '', last_editor INTEGER NULL REFERENCES people(id), last_edit_time datetime DEFAULT CURRENT_TIMESTAMP)")
		checkErr(err)
	}
	if _, ok := tables["sessions"]; !ok {
		log.Print("creating sessions table")
		stmt, err := db.Prepare("CREATE TABLE sessions (id text PRIMARY KEY, person_id INTEGER REFERENCES people(id), create_time DATETIME DEFAULT CURRENT_TIMESTAMP)")
		defer stmt.Close()
		_, err = stmt.Exec()
		checkErr(err)
	}
}

func GetUsers() ([]*Person, error) {
	log.Print("GetUsers")
	if conn == nil {
		log.Panic("Database was not open")
	}

	rows, err := conn.Query(`SELECT p.id, p.username, p.name, p.department, p.status, p.notes, p.telephone, p.mobile, p.office, p.title, l.name, p.last_edit_time
		FROM people p
		LEFT JOIN people l ON p.last_editor = l.id
		ORDER BY p.department, p.name`)
	defer rows.Close()
	checkErr(err)
	if err != nil {
		return nil, err
	}

	var id int
	var username string
	var name string
	var department sql.NullString
	var notes string
	var status int
	var telephone string
	var mobile string
	var office string
	var people []*Person
	var lastEditor sql.NullString
	var lastEditTime NullTime
	var title string
	statuses, err := StatusCodes()
	if err != nil {
		log.Fatalf("Failed to get status codes from the database: %s", err)
	}

	for rows.Next() {
		err = rows.Scan(
			&id,
			&username,
			&name,
			&department,
			&status,
			&notes,
			&telephone,
			&mobile,
			&office,
			&title,
			&lastEditor,
			&lastEditTime)
		checkErr(err)
		if err != nil {
			return nil, err
		}
		p := &Person{
			ID:         id,
			Name:       name,
			Username:   username,
			Department: department.String,
			Status:     statuses[status],
			Remarks:    notes,
			Telephone:  telephone,
			Mobile:     mobile,
			Office:     office,
			Title:      title,
			LastEditor: "",
		}

		p.Status = statuses[status]

		if lastEditor.Valid {
			p.LastEditor = lastEditor.String
		}
		if lastEditTime.Valid {
			p.LastEditTime = lastEditTime.Time.Local()
		}
		people = append(people, p)
	}
	log.Debugf("Got %d people from the db\n", len(people))
	rows.Close()

	return people, nil
}

func GetPerson(username string) (*Person, error) {
	log.Print("GetPerson")
	if conn == nil {
		log.Panic("Database was not open")
	}
	var person *Person
	var id int
	var uname string
	var name string
	var status int
	var notes string
	var department string
	var office sql.NullString
	var telephone sql.NullString
	var mobile sql.NullString
	var title string
	var lastEditor sql.NullString
	var lastEditTime NullTime

	stmt, err := conn.Prepare(`SELECT p.id, p.username, p.name, p.department, p.status, p.notes, p.telephone, p.mobile, p.office, p.title, l.name as last_editor, p.last_edit_time
	FROM people p left join people l on l.id = p.last_editor WHERE p.username = ?`)
	defer stmt.Close()
	checkErr(err)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(username)
	defer rows.Close()
	checkErr(err)
	if err != nil {
		return nil, err
	}
	statuses, err := StatusCodes()
	if err != nil {
		log.Fatalf("Could not get status codes from the database: %s", err)
	}

	if rows.Next() {
		err = rows.Scan(&id, &uname, &name, &department, &status, &notes, &telephone, &mobile, &office, &title, &lastEditor, &lastEditTime)
		checkErr(err)
		if err != nil {
			return nil, err
		}
		person = &Person{
			ID:         id,
			Name:       name,
			Username:   username,
			Department: department,
			Title:      title,
			Remarks:    notes,
		}
		person.Status = statuses[status]

		if lastEditor.Valid {
			person.LastEditor = lastEditor.String
		}
		if lastEditTime.Valid {
			person.LastEditTime = lastEditTime.Time.Local()
		}
		if telephone.Valid {
			person.Telephone = telephone.String
		}
		if mobile.Valid {
			person.Mobile = mobile.String
		}
		if office.Valid {
			person.Office = office.String
		}
	} else {
		err = fmt.Errorf("No user named %s", username)
		person = nil
	}
	rows.Close()

	return person, err
}

func SetPerson(person *Person, username string) error {
	stmt, err := conn.Prepare("UPDATE people SET status = ?, notes = ?, last_editor = (select id from people where username = ?), last_edit_time = current_timestamp WHERE username = ?")
	defer stmt.Close()
	checkErr(err)
	res, err := stmt.Exec(person.Status.Code, person.Remarks, username, person.Username)
	checkErr(err)

	rows, err := res.RowsAffected()
	checkErr(err)
	if rows != 1 {
		err = fmt.Errorf("Failed to update user %s", person.Username)
	} else {
		err = nil
	}

	return err
}

// Updates details for a user. This is meant to be used
// internally for attributes that are not editable by
// the user.
func SetPersonDetails(person *Person) error {
	stmt, err := conn.Prepare("UPDATE people SET name = ?, department = ?, telephone = ?, mobile = ?, office = ?, title = ? WHERE username = ?")
	defer stmt.Close()
	checkErr(err)
	res, err := stmt.Exec(person.Name, person.Department, person.Telephone, person.Mobile, person.Office, person.Title, person.Username)
	checkErr(err)
	rows, err := res.RowsAffected()
	checkErr(err)
	if rows != 1 {
		err = fmt.Errorf("Failed to update user %s", person.Username)
	} else {
		err = nil
	}

	return err
}

func StatusCodes() (map[int]Status, error) {
	if statusCodes == nil {
		statusCodes = make(map[int]Status)
	}
	mutex.Lock()
	if len(statusCodes) > 0 {
		mutex.Unlock()
		return statusCodes, nil
	}
	stmt, err := conn.Prepare("SELECT * FROM status")
	defer stmt.Close()
	checkErr(err)
	if err != nil {
		statusCodes = make(map[int]Status)
		mutex.Unlock()
		return statusCodes, err
	}
	rows, err := stmt.Query()
	defer rows.Close()
	checkErr(err)
	if err != nil {
		statusCodes = make(map[int]Status)
		mutex.Unlock()
		return statusCodes, err
	}

	var status Status
	for rows.Next() {
		status = Status{}
		err = rows.Scan(&status.Code, &status.Value)
		statusCodes[status.Code] = status
		if err != nil {
			statusCodes = make(map[int]Status)
			mutex.Unlock()
			return statusCodes, err
		}
	}
	mutex.Unlock()
	return statusCodes, nil
}

// Remove a person from the database
func RemovePerson(person *Person) error {
	stmt, err := conn.Prepare("DELETE FROM people WHERE username = ?")
	defer stmt.Close()
	res, err := stmt.Exec(person.Username)
	checkErr(err)
	_,err = res.RowsAffected()
	checkErr(err)
	return err
}

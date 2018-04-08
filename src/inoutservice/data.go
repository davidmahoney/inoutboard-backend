package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

var conn *sql.DB

func init() {
	log.Println("Creating database...")
	createDb()
}

func ValidateSession(sessionID string) (string, error) {
	stmt, err := conn.Prepare("SELECT username FROM sessions JOIN people ON (person_id = people.id) WHERE sessions.id = ?")
	checkErr(err)
	res, err := stmt.Query(sessionID)
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
	checkErr(err)
	stmt.Exec(sessionID, userID)
	checkErr(err)
	return err
}

func RemoveSession(sessionID string) error {
	stmt, err := conn.Prepare("DELETE FROM sessions WHERE id = ?")
	checkErr(err)
	_, err = stmt.Exec(sessionID)
	checkErr(err)
	return err
}

func checkErr(err error) {
	if err != nil {
		fmt.Printf(err.Error())
		fmt.Println(" Noooooo!\n")
	}
}

func AddPerson(username string, name string, department string, telephone string, mobile string, office string) error {
	stmt, err := conn.Prepare("INSERT INTO people (username, name, status, department, mobile, telephone, office) VALUES (?,?,?,?,?,?,?)")
	if err != nil {
		log.Fatal(err)
	}

	_, err = stmt.Exec(username, name, Out, department, mobile, telephone, office)
	log.Printf("Added %s to the db", username)
	return err
}

func createDb() {
	db, err := sql.Open("sqlite3", "db.sqlite")
	checkErr(err)
	conn = db

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table'")
	tables := make(map[string]string)
	var table string
	checkErr(err)
	for rows.Next() {
		err = rows.Scan(&table)
		checkErr(err)
		log.Printf("found table %s", table)
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
		res, err = stmt.Exec("InField")
		checkErr(err)
	}
	log.Print("creating people table")
	if _, ok := tables["people"]; !ok {
		_, err = db.Exec("CREATE TABLE people (id INTEGER PRIMARY KEY, username TEXT UNIQUE, name TEXT NOT NULL, department TEXT null, mobile TEXT not null default '', telephone TEXT not null default '', office TEXT not null default '', status int REFERENCES status(id), notes TEXT DEFAULT '', last_editor INTEGER NULL REFERENCES people(id), last_edit_time datetime DEFAULT CURRENT_TIMESTAMP)")
		checkErr(err)
	}
	if _, ok := tables["sessions"]; !ok {
		log.Print("creating sessions table")
		stmt, err := db.Prepare("CREATE TABLE sessions (id text PRIMARY KEY, person_id INTEGER REFERENCES people(id), create_time DATETIME DEFAULT CURRENT_TIMESTAMP)")
		_, err = stmt.Exec()
		checkErr(err)
	}
}

func GetUsers() ([]*Person, error) {
	log.Print("GetUsers")
	if conn == nil {
		createDb()
	}

	rows, err := conn.Query(`SELECT p.id, p.username, p.name, p.department, p.status, p.notes, p.telephone, p.mobile, p.office, l.name, p.last_edit_time
		FROM people p
		LEFT JOIN people l ON p.last_editor = l.id
		ORDER BY p.department, p.name`)
	checkErr(err)

	var id int
	var username string
	var name string
	var department sql.NullString
	var notes string
	var status Status
	var telephone string
	var mobile string
	var office string
	var people []*Person
	var statusValue string = "Out"
	var lastEditor sql.NullString
	var lastEditTime NullTime

	for rows.Next() {
		err = rows.Scan(&id, &username, &name, &department, &status, &notes, &telephone, &mobile, &office, &lastEditor, &lastEditTime)
		switch status {
		case In:
			statusValue = "In"
		case Out:
			statusValue = "Out"
		case InField:
			statusValue = "In Field"
		}
		checkErr(err)
		p := &Person{
			ID:          id,
			Name:        name,
			Username:    username,
			Department:  department.String,
			Status:      status,
			StatusValue: statusValue,
			Remarks:     notes,
			Telephone:   telephone,
			Mobile:      mobile,
			Office:      office,
			LastEditor:  "",
		}
		if lastEditor.Valid {
			p.LastEditor = lastEditor.String
		}
		if lastEditTime.Valid {
			p.LastEditTime = lastEditTime.Time.Local()
		}
		people = append(people, p)
	}
	fmt.Printf("Got %d people from the db\n", len(people))
	rows.Close()

	return people, nil
}

func GetPerson(username string) (*Person, error) {
	log.Print("GetPerson")
	if conn == nil {
		createDb()
	}
	var person *Person
	var id int
	var uname string
	var name string
	var status Status
	var notes string
	var department string
	var statusValue string = "Out"
	var office sql.NullString
	var telephone sql.NullString
	var mobile sql.NullString
	var lastEditor sql.NullString
	var lastEditTime NullTime

	stmt, err := conn.Prepare(`SELECT p.id, p.username, p.name, p.department, p.status, p.notes, p.telephone, p.mobile, p.office, l.name as last_editor, p.last_edit_time
	FROM people p left join people l on l.id = p.last_editor WHERE p.username = ?`)
	checkErr(err)
	rows, err := stmt.Query(username)
	checkErr(err)

	if rows.Next() {
		err = rows.Scan(&id, &uname, &name, &department, &status, &notes, &telephone, &mobile, &office, &lastEditor, &lastEditTime)
		checkErr(err)
		switch status {
		case In:
			statusValue = "In"
		case Out:
			statusValue = "Out"
		case InField:
			statusValue = "In Field"
		}
		person = &Person{
			ID:          id,
			Name:        name,
			Username:    username,
			Department:  department,
			Status:      status,
			StatusValue: statusValue,
			Remarks:     notes,
		}
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
	checkErr(err)
	res, err := stmt.Exec(person.Status, person.Remarks, username, person.Username)
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
	stmt, err := conn.Prepare("UPDATE people SET name = ?, department = ?, telephone = ?, mobile = ?, office = ? WHERE username = ?")
	checkErr(err)
	res, err := stmt.Exec(person.Name, person.Department, person.Telephone, person.Mobile, person.Office, person.Username)
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

package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

var conn *sql.DB

func Init() {
	createDb()
}

func checkErr(err error) {
	if err != nil {
		fmt.Println("Noooooo!\n")
		panic(err)
	}
}

func createDb() {
	db, err := sql.Open("sqlite3", ":memory:")
	checkErr(err)
	conn = db
	res, err := db.Exec("CREATE TABLE status (id INTEGER PRIMARY KEY, value TEXT)")
	checkErr(err)
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
	res, err = db.Exec("CREATE TABLE people (id INTEGER PRIMARY KEY, username TEXT UNIQUE, name TEXT, status int REFERENCES status(id), notes TEXT)")
	stmt, err = db.Prepare("INSERT INTO people (username, name, status, notes) VALUES (?,?,?,?)")
	stmt.Exec("eartburm", "David", 0, "Blarg!")
	checkErr(err)

}

func GetUsers() ([]*Person, error) {
	if conn == nil {
		createDb()
	}

	rows, err := conn.Query("SELECT * FROM people")
	checkErr(err)

	var id int
	var username string
	var name string
	var notes string
	var status Status
	var people []*Person
	var statusValue string = "Out"

	for rows.Next() {
		err = rows.Scan(&id, &username, &name, &status, &notes)
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
			Status:      status,
			StatusValue: statusValue,
			Remarks:     notes}
		people = append(people, p)
	}
	fmt.Printf("Got %d people from the db\n", len(people))
	rows.Close()

	return people, nil
}

func GetPerson(username string) (*Person, error) {
	if conn == nil {
		createDb()
	}
	var person *Person
	var id int
	var uname string
	var name string
	var status Status
	var notes string
	var statusValue string = "Out"

	stmt, err := conn.Prepare("SELECT * FROM people WHERE username = ?")
	checkErr(err)
	rows, err := stmt.Query(username)
	checkErr(err)

	if rows.Next() {
		err = rows.Scan(&id, &uname, &name, &status, &notes)
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
			Status:      status,
			StatusValue: statusValue,
			Remarks:     notes,
		}
	} else {
		err = fmt.Errorf("No user named %s", username)
		person = nil
	}
	rows.Close()

	return person, err
}

func SetPerson(person *Person) error {
	stmt, err := conn.Prepare("UPDATE people SET status = ?, notes = ? WHERE username = ?")
	checkErr(err)
	res, err := stmt.Exec(person.Status, person.Remarks, person.Username)
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

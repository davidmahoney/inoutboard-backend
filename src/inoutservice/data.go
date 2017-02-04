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
	res, err = db.Exec("CREATE TABLE people (id INTEGER PRIMARY KEY, username TEXT, name TEXT, status int REFERENCES status(id), notes TEXT)")
	stmt, err = db.Prepare("INSERT INTO people (username, name, status, notes) VALUES (?,?,?,?)")
	stmt.Exec("eartburm", "David", 0, "Blarg!")
	checkErr(err)

}

func GetUsers() []Person {
	if conn == nil {
		createDb()
	}

	rows, err := conn.Query("SELECT * FROM people")
	checkErr(err)

	var people []Person
	var id int
	var username string
	var name string
	var notes string
	var status Status

	for rows.Next() {
		err = rows.Scan(&id, &username, &name, &status, &notes)
		checkErr(err)
		p := Person{
			Name:     name,
			Username: username,
			Status:   status,
			Remarks:  notes}
		people = append(people, p)
	}
	rows.Close()

	return people
}

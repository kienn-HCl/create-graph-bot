package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func init() {
	log.Println("setup database...")
	var err error
	db, err = sql.Open("sqlite3", "database.sqlite")
	if err != nil {
		log.Fatalln("error setup DB:", err)
	}
}

func GetDBTables() (tables []string, err error) {
	rows, err := db.Query(`
        SELECT name FROM sqlite_master WHERE type = 'table';
    `)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var table string
		err = rows.Scan(&table)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func SetDBTable(table string) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ?(time TEXT, data JSON, PRIMARY KEY(time))
    `,
		table,
	)
	if err != nil {
		return err
	}
	return nil
}

func AddJSONDataToDB(table string, datetime time.Time, json string) error {
	_, err := db.Exec(`
        REPLACE INTO ?(time, data) values(?,?)
    `,
		table,
		datetime.UTC().Format(time.DateTime),
		json,
	)
	if err != nil {
		return err
	}

	return nil
}

func GetJSONDataFromDB(table, key string, before, after time.Time) (data map[time.Time]string, err error) {
	rows, err := db.Query(`
        SELECT
            time
            data->>'?'
        FROM
            ?
        WHERE
            ? >= time AND
            time <= ?
    `,
		key,
		table,
		before.UTC().Format(time.DateTime),
		after.UTC().Format(time.DateTime),
	)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var datetime, elem string
		err = rows.Scan(&datetime, &elem)
		if err != nil {
			return nil, err
		}

		elemTime, err := time.Parse(time.DateTime, datetime)
		if err != nil {
			return nil, err
		}
		data[elemTime] = elem
	}
	return data, nil
}


package main

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS sensor_data_json(time TEXT, data JSON, PRIMARY KEY(time))`,
	}
	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func addJSONdata(db *sql.DB, time time.Time, json string) error {
	_, err := db.Exec(`
        REPLACE INTO sensor_data_json(time, data) values(?,?)
    `,
		time,
		json,
	)
	if err != nil {
		return err
	}

	return nil
}

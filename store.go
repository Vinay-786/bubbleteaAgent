package main

import (
	"database/sql"
	"log"

	"github.com/cloudflare/cloudflare-go/v4/ai"
	_ "modernc.org/sqlite"
)

type sqliteStore struct {
	db *sql.DB
}

func (s *sqliteStore) Init() error {
	var err error
	s.db, err = sql.Open("sqlite", "data.db")
	if err != nil {
		return err
	}
	return nil
}

func (s *sqliteStore) createTables() {
	createconvo := `
		CREATE TABLE IF NOT EXISTS sessions (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`
	if _, err := s.db.Exec(createconvo); err != nil {
		log.Fatal(err)
	}

	createsession := `
		CREATE TABLE IF NOT EXISTS conversations (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    session_id INTEGER NOT NULL,
	    role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
	    content TEXT NOT NULL,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		); `
	if _, err := s.db.Exec(createsession); err != nil {
		log.Fatal(err)
	}
}

func (s *sqliteStore) SaveConversation(c []ai.AIRunParamsBodyTextGenerationMessage) error {
	res, err := s.db.Exec(`INSERT INTO sessions DEFAULT VALUES`)
	if err != nil {
		return err
	}

	sessionID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	insertQuery := `INSERT INTO conversations (session_id, role, content) VALUES (?, ?, ?)`
	stmt, err := s.db.Prepare(insertQuery)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msg := range c {
		_, err := stmt.Exec(sessionID, msg.Role.String(), msg.Content.String())
		if err != nil {
			return err
		}
	}
	return nil
}

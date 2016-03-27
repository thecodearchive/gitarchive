package queue

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// Queue implements a simple de-duplicating queue that assumes that when a
// consumer runs Pop it will finish its job, and that all the Add calls up to
// the Pop call are fulfilled.
//
// It is safe for concurrent use by multiple goroutines AND processes.
//
// The Queue keeps no memory of Pop-ed names, so the populator is supposed to
// know when a Pop happened more recently than the event triggering the Add.
type Queue struct {
	db *sql.DB

	insertQ *sql.Stmt
	selectQ *sql.Stmt
	deleteQ *sql.Stmt
}

func Open(path string) (*Queue, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	q := &Queue{db: db}

	query := `CREATE TABLE IF NOT EXISTS Queue (
        ID INTEGER PRIMARY KEY AUTOINCREMENT, Name TEXT UNIQUE NOT NULL, Parent TEXT)`
	if _, err = db.Exec(query); err != nil {
		return nil, err
	}

	query = `INSERT OR IGNORE INTO Queue (Name, Parent) VALUES (?, ?)`
	if q.insertQ, err = db.Prepare(query); err != nil {
		return nil, err
	}

	query = `SELECT ID, Name, Parent FROM Queue ORDER BY ID ASC LIMIT 1`
	if q.selectQ, err = db.Prepare(query); err != nil {
		return nil, err
	}

	query = `DELETE FROM Queue WHERE ID = ?`
	if q.deleteQ, err = db.Prepare(query); err != nil {
		return nil, err
	}

	return q, nil
}

// Add is idempotent
func (q *Queue) Add(name, parent string) error {
	_, err := q.insertQ.Exec(name, parent)
	return err
}

// Pop returns "", "", nil when the queue is empty
func (q *Queue) Pop() (name, parent string, err error) {
	tx, err := q.db.Begin()
	if err != nil {
		return "", "", err
	}
	defer func() { err = tx.Commit() }()

	var id int
	err = tx.Stmt(q.selectQ).QueryRow().Scan(&id, &name, &parent)
	if err == sql.ErrNoRows {
		return "", "", nil
	} else if err != nil {
		return "", "", err
	}

	_, err = tx.Stmt(q.deleteQ).Exec(id)
	if err != nil {
		return "", "", err
	}

	return
}

func (q *Queue) Close() error {
	// Do we need to close the Stmt here?
	return q.db.Close()
}

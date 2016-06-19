package queue

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
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
	countQ  *sql.Stmt
}

func Open(dataSourceName string) (*Queue, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, err
	}

	q := &Queue{db: db}

	query := `CREATE TABLE IF NOT EXISTS Queue (
		ID INTEGER PRIMARY KEY AUTO_INCREMENT, Name VARCHAR(256) UNIQUE NOT NULL, Parent VARCHAR(256))`
	if _, err = db.Exec(query); err != nil {
		return nil, fmt.Errorf("table creation failed: %s", err)
	}

	query = `INSERT INTO Queue (Name, Parent) VALUES (?, ?) ON DUPLICATE KEY UPDATE Name=VALUES(Name)`
	if q.insertQ, err = db.Prepare(query); err != nil {
		return nil, fmt.Errorf("insert preparation failed: %s", err)
	}

	query = `SELECT ID, Name, Parent FROM Queue ORDER BY ID ASC LIMIT 1`
	if q.selectQ, err = db.Prepare(query); err != nil {
		return nil, fmt.Errorf("select preparation failed: %s", err)
	}

	query = `DELETE FROM Queue WHERE ID = ?`
	if q.deleteQ, err = db.Prepare(query); err != nil {
		return nil, fmt.Errorf("delete preparation failed: %s", err)
	}

	query = `SELECT COUNT(*) FROM Queue`
	if q.countQ, err = db.Prepare(query); err != nil {
		return nil, fmt.Errorf("select preparation failed: %s", err)
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

func (q *Queue) Len() (int, error) {
	var res int
	err := q.countQ.QueryRow().Scan(&res)
	return res, err
}

func (q *Queue) Close() error {
	// Do we need to close the Stmt here?
	return q.db.Close()
}

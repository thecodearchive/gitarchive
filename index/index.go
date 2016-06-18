package index

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

type Index struct {
	db *sql.DB

	insertFetchQ, insertDepQ *sql.Stmt
	selectQ                  *sql.Stmt
}

func Open(dataSourceName string) (*Index, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, err
	}

	i := &Index{db: db}

	query := `CREATE TABLE IF NOT EXISTS Fetches (
		Name VARCHAR(255) NOT NULL INDEX, Parent VARCHAR(255),
		Timestamp DATETIME, Refs JSON,
		PackID BIGINT UNIQUE INDEX AUTO_INCREMENT, PackRef VARCHAR(255))`
	if _, err = db.Exec(query); err != nil {
		return nil, errors.Wrap(err, "failed to create Fetches")
	}
	query = `CREATE TABLE IF NOT EXISTS PackDeps (ID BIGINT INDEX, Dep BIGINT)`
	if _, err = db.Exec(query); err != nil {
		return nil, errors.Wrap(err, "failed to create PackDeps")
	}

	query = `INSERT INTO Fetches (Name, Parent, Timestamp, Refs, PackRef)
		VALUES (?, ?, ?, ?, ?)`
	if i.insertFetchQ, err = db.Prepare(query); err != nil {
		return nil, errors.Wrap(err, "failed to prepare INSERT")
	}
	query = `INSERT INTO PackDeps (ID, Dep) VALUES (?, ?)`
	if i.insertDepQ, err = db.Prepare(query); err != nil {
		return nil, errors.Wrap(err, "failed to prepare INSERT")
	}

	query = `SELECT Parent, Refs, PackID FROM Fetches WHERE Name = ?
		ORDER BY Timestamp DESC LIMIT 1`
	if i.selectQ, err = db.Prepare(query); err != nil {
		return nil, errors.Wrap(err, "failed to prepare SELECT")
	}

	return i, nil
}

func (i *Index) AddFetch(name, parent string, timestamp time.Time,
	refs map[string]string, packRef string, packDeps []string) error {
	r, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	_, err = i.insertFetchQ.Exec(name, parent, timestamp, r, packRef)
	if err != nil {
		return err
	}
	for _, dep := range packDeps {
		_, err := i.insertDepQ.Exec(packRef, dep)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Index) GetHaves(name string) (haves map[string]struct{}, deps []string, err error) {
	var parent, packID string
	var refs []byte
	err = i.selectQ.QueryRow(name).Scan(&parent, &refs, &packID)
	if err == sql.ErrNoRows {
		err = nil
		return
	}
	if err != nil {
		return
	}
	var r map[string]string
	err = json.Unmarshal(refs, &r)
	if err != nil {
		return
	}

	haves = make(map[string]struct{})
	for ref := range r {
		haves[ref] = struct{}{}
	}
	deps = append(deps, packID)

	if parent != "" {
		err = i.selectQ.QueryRow(parent).Scan(&parent, &refs, &packID)
		if err == sql.ErrNoRows {
			err = nil
			return
		}
		if err != nil {
			return
		}
		err = json.Unmarshal(refs, &r)
		if err != nil {
			return
		}

		for ref := range r {
			haves[ref] = struct{}{}
		}
		deps = append(deps, packID)
	}

	return
}

func (i *Index) Close() error {
	return i.db.Close()
}

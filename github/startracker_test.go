package github

import (
	"database/sql"
	"database/sql/driver"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

type AnyInt struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyInt) Match(v driver.Value) bool {
	_, ok := v.(int64)
	return ok
}

func TestStarTracker(t *testing.T) {
	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		t.Fatal("Please set the env var GITHUB_TOKEN")
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Creating a new StarTracker, we expect to create a table and prepare 3 statements
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectPrepare("SELECT")
	mock.ExpectPrepare("SELECT")
	mock.ExpectPrepare("INSERT")
	st, err := NewStarTracker(db, ghToken)
	if err != nil {
		t.Fatalf("error occured creating a new StarTracker: %v", err)
	}

	// Getting a repo not in the database, we expect to Get to fail to find the row,
	// fetch the information from GitHub, and insert it into the database
	mock.ExpectQuery("SELECT").WithArgs("FiloSottile/ansible-sshknownhosts").
		WillReturnError(sql.ErrNoRows)
	// Note: this expectation is broken if the repo is starred.
	mock.ExpectExec("INSERT").WithArgs("FiloSottile/ansible-sshknownhosts", AnyInt{}, "bfmartin/ansible-sshknownhosts", AnyTime{}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	starsOld, parent, err := st.Get("FiloSottile/ansible-sshknownhosts")
	if err != nil {
		t.Fatalf("error getting FiloSottile/ansible-sshknownhosts: %v", err)
	}
	if parent != "bfmartin/ansible-sshknownhosts" {
		t.Fatalf("returned parent %s does not match bfmartin/ansible-sshknownhosts", parent)
	}

	// Receiving a WatchEvent that was created before the last updated time shouldn't do anything
	st.panicIfNetwork = true
	row := sqlmock.NewRows([]string{"Name", "Stars", "Parent", "LastUpdated"}).
		AddRow("FiloSottile/ansible-sshknownhosts", starsOld, "bfmartin/ansible-sshknownhosts", time.Now())
	mock.ExpectQuery("SELECT").WithArgs("FiloSottile/ansible-sshknownhosts").WillReturnRows(row)
	err = st.WatchEvent("FiloSottile/ansible-sshknownhosts", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("watchevent went wrong: %v", err)
	}

	// Receiving a WatchEvent that was created after the last updated time should update the row with an incremented star count
	row = sqlmock.NewRows([]string{"Name", "Stars", "Parent", "LastUpdated"}).
		AddRow("FiloSottile/ansible-sshknownhosts", 0, "bfmartin/ansible-sshknownhosts", time.Now())
	mock.ExpectQuery("SELECT").WithArgs("FiloSottile/ansible-sshknownhosts").WillReturnRows(row)
	mock.ExpectExec("INSERT").WithArgs("FiloSottile/ansible-sshknownhosts", starsOld+1, "bfmartin/ansible-sshknownhosts", AnyTime{}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = st.WatchEvent("FiloSottile/ansible-sshknownhosts", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("watchevent went wrong: %v", err)
	}

	// Creating an event with a new repo should insert it into the database
	mock.ExpectQuery("SELECT").WithArgs("FiloSottile/foo").WillReturnError(sql.ErrNoRows)
	// The following expectation will break if FiloSottile/foo has any stars
	mock.ExpectExec("INSERT").WithArgs("FiloSottile/foo", 0, "", AnyTime{}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = st.CreateEvent("FiloSottile/foo", "", time.Now())
	if err != nil {
		t.Fatalf("could not create event: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

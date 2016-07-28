package github

import (
	"database/sql"
	"errors"
	"expvar"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// StarTracker keeps track of how many stars a repository has. It is backed by a
// db on disk, goes to GitHub for never-seen-before, and it assumes it will
// be told about every WatchEvent ever since so that it can keep the number
// accurate without ever going to the network again.
//
// A StarTracker is not safe to use by multiple goroutines at the same time, and
// it assumes WatchEvents and Gets are submited sequentially anyway. However, it
// is fully idempotent.
type StarTracker struct {
	db *sql.DB
	gh *github.Client

	exp          *expvar.Map
	expRateLeft  *expvar.Int
	expRateReset *expvar.String

	setR, getR, numRows *sql.Stmt

	panicIfNetwork bool // used for testing
}

type Repo struct {
	Name        string
	Stars       int
	Parent      string
	LastUpdated time.Time
}

func NewStarTracker(db *sql.DB, gitHubToken string) (*StarTracker, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitHubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	s := &StarTracker{db: db, gh: github.NewClient(tc)}

	query := `CREATE TABLE IF NOT EXISTS StarTracker (
		Name VARCHAR(255) NOT NULL PRIMARY KEY, Stars INT,
		Parent VARCHAR(255), LastUpdated DATETIME)`
	if _, err := db.Exec(query); err != nil {
		return nil, fmt.Errorf("couldn't create stars table: %v", err)
	}

	// Prepare SQL Queries
	prepStmts := []struct {
		name **sql.Stmt
		sql  string
	}{
		{
			&s.setR,
			`INSERT INTO StarTracker(Name, Stars, Parent, LastUpdated) VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE Name = Values(Name), Stars = Values(Stars), Parent = Values(Parent), LastUpdated = Values(LastUpdated)`,
		},
		{
			&s.getR,
			`SELECT Name, Stars, Parent, LastUpdated FROM StarTracker WHERE Name = ?`,
		},
		{
			&s.numRows,
			`SELECT COUNT(*) FROM StarTracker`,
		},
	}

	for _, prepStmt := range prepStmts {
		stmt, err := db.Prepare(prepStmt.sql)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare %s: %v'", prepStmt.sql, err)
		}
		*prepStmt.name = stmt
	}

	s.gh.UserAgent = "github.com/thecodearchive/gitarchive/github StarTracker"

	s.exp = new(expvar.Map).Init()
	s.expRateLeft = new(expvar.Int)
	s.expRateReset = new(expvar.String)
	s.exp.Set("rateleft", s.expRateLeft)
	s.exp.Set("ratereset", s.expRateReset)
	s.exp.Set("cachesize", expvar.Func(func() interface{} {
		var n int
		s.numRows.QueryRow().Scan(&n)
		return n
	}))

	return s, nil
}

func (s *StarTracker) getRepo(key string) (r *Repo, err error) {
	r = &Repo{}
	err = s.getR.QueryRow(key).Scan(&r.Name, &r.Stars, &r.Parent, &r.LastUpdated)
	if err == sql.ErrNoRows {
		r = nil
		err = nil
	}

	return
}

func (s *StarTracker) setRepo(key string, r *Repo) error {
	_, err := s.setR.Exec(r.Name, r.Stars, r.Parent, r.LastUpdated)
	return err
}

func (s *StarTracker) Get(name string) (stars int, parent string, err error) {
	rr, err := s.getRepo(name)
	if err != nil {
		return 0, "", err
	}

	if rr != nil {
		s.exp.Add("cachehits", 1)
		return rr.Stars, rr.Parent, nil
	}

	if s.panicIfNetwork {
		panic("network connection with panicIfNetwork=true")
	}

	nameParts := strings.Split(name, "/")
	if len(nameParts) != 2 {
		return 0, "", errors.New("name must be in user/repo format")
	}

	t := time.Now()
	s.exp.Add("apicalls", 1)
	r, hr, err := s.gh.Repositories.Get(nameParts[0], nameParts[1])
	logGHRateReset(hr)
	s.trackRate()
	if err != nil {
		return 0, "", err
	}
	if r.StargazersCount == nil {
		return 0, "", errors.New("GitHub didn't tell us the StargazersCount")
	}

	if r.Parent != nil && r.Parent.FullName != nil {
		parent = *r.Parent.FullName
	}

	return *r.StargazersCount, parent, s.setRepo(name, &Repo{
		Name:        name,
		Stars:       *r.StargazersCount,
		LastUpdated: t,
		Parent:      parent,
	})
}

func (s *StarTracker) WatchEvent(name string, created time.Time) error {
	repo, err := s.getRepo(name)
	if err != nil {
		return err
	}
	if repo == nil {
		return nil
	}

	if created.After(repo.LastUpdated) {
		repo.Stars++
		repo.LastUpdated = created
		return s.setRepo(name, repo)
	}
	return nil
}

func (s *StarTracker) CreateEvent(name, parent string, created time.Time) error {
	if repo, err := s.getRepo(name); err != nil || repo != nil {
		return err // maintain idempotency
	}
	return s.setRepo(name, &Repo{
		Name:        name,
		Stars:       0,
		LastUpdated: created,
		Parent:      parent,
	})
}

func (s *StarTracker) Expvar() *expvar.Map {
	return s.exp
}

func (s *StarTracker) trackRate() {
	rate := s.gh.Rate()
	s.expRateLeft.Set(int64(rate.Remaining))
	s.expRateReset.Set(rate.Reset.String())
}

func Is404(err error) bool {
	if err, ok := err.(*github.ErrorResponse); ok {
		if err.Response != nil && err.Response.StatusCode == 404 {
			return true
		}
	}
	return false
}

func IsRateLimit(err error) *github.Rate {
	if err, ok := err.(*github.RateLimitError); ok {
		return &err.Rate
	}
	return nil
}

package github

import (
	"errors"
	"expvar"
	"strings"
	"time"

	"github.com/boltdb/bolt"
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
	db *bolt.DB
	gh *github.Client

	exp          *expvar.Map
	expRateLeft  *expvar.Int
	expRateReset *expvar.String

	panicIfNetwork bool // used for testing
}

//go:generate msgp -io=false -tests=false -unexported
//msgp:ignore StarTracker

type Repo struct {
	Stars       int
	Parent      string
	LastUpdated time.Time
}

func NewStarTracker(db *bolt.DB, gitHubToken string) *StarTracker {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitHubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("StarTracker"))
		return nil
	})

	s := &StarTracker{db: db, gh: github.NewClient(tc)}
	s.gh.UserAgent = "github.com/thecodearchive/gitarchive/github StarTracker"

	s.exp = new(expvar.Map).Init()
	s.expRateLeft = new(expvar.Int)
	s.expRateReset = new(expvar.String)
	s.exp.Set("rateleft", s.expRateLeft)
	s.exp.Set("ratereset", s.expRateReset)
	s.exp.Set("cachesize", expvar.Func(func() interface{} {
		var n int
		s.db.View(func(tx *bolt.Tx) error {
			n = tx.Bucket([]byte("StarTracker")).Stats().KeyN
			return nil
		})
		return n
	}))

	return s
}

func (s *StarTracker) getRepo(key string) (r *Repo, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("StarTracker"))
		v := b.Get([]byte(key))
		if v == nil {
			return nil
		}
		r = &Repo{}
		_, err := r.UnmarshalMsg(v)
		return err
	})
	return
}

func (s *StarTracker) setRepo(key string, r *Repo) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("StarTracker"))
		v, err := r.MarshalMsg(nil)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), v)
	})
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
		repo.Stars += 1
		repo.LastUpdated = created
	}
	return s.setRepo(name, repo)
}

func (s *StarTracker) CreateEvent(name, parent string, created time.Time) error {
	if repo, err := s.getRepo(name); err != nil || repo != nil {
		return err // maintain idempotency
	}
	return s.setRepo(name, &Repo{
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

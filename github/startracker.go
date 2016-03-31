package github

import (
	"encoding/json"
	"errors"
	"expvar"
	"io"
	"strings"
	"time"

	"github.com/thecodearchive/gitarchive/lru"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// StarTracker keeps track of how many stars a repository has. It keeps a huge
// in-memory LRU, goes to GitHub for never-seen-before, and it assumes it will
// be told about every WatchEvent ever since so that it can keep the number
// accurate without ever going to the network again.
//
// A StarTracker is not safe to use by multiple goroutines at the same time, and
// it assumes WatchEvents and Gets are submited sequentially anyway. However, it
// is fully idempotent.
type StarTracker struct {
	lru *lru.Cache
	gh  *github.Client

	exp          *expvar.Map
	expRateLeft  *expvar.Int
	expRateReset *expvar.String

	panicIfNetwork bool // used for testing
}

type repo struct {
	Stars       int
	Parent      string
	LastUpdated time.Time
}

func NewStarTracker(maxSize int, gitHubToken string) *StarTracker {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitHubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	s := &StarTracker{lru: lru.New(maxSize), gh: github.NewClient(tc)}

	s.exp = new(expvar.Map).Init()
	s.expRateLeft = new(expvar.Int)
	s.expRateReset = new(expvar.String)
	s.exp.Set("rateleft", s.expRateLeft)
	s.exp.Set("ratereset", s.expRateReset)
	s.exp.Set("cachesize", expvar.Func(func() interface{} { return s.lru.Len() }))

	return s
}

func (s *StarTracker) Get(name string) (stars int, parent string, err error) {
	res, ok := s.lru.Get(name)
	if ok {
		s.exp.Add("cachehits", 1)
		repo := res.(*repo)
		return repo.Stars, repo.Parent, nil
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
	logGHRateReset(hr.Response)
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

	s.lru.Add(name, &repo{
		Stars:       *r.StargazersCount,
		LastUpdated: t,
		Parent:      parent,
	})
	return *r.StargazersCount, parent, nil
}

func (s *StarTracker) WatchEvent(name string, created time.Time) {
	res, ok := s.lru.Get(name)
	if !ok {
		return
	}

	repo := res.(*repo)
	if created.After(repo.LastUpdated) {
		repo.Stars += 1
		repo.LastUpdated = created
	}
}

func (s *StarTracker) CreateEvent(name, parent string, created time.Time) {
	if _, ok := s.lru.Get(name); ok {
		return // maintain idempotency
	}
	s.lru.Add(name, &repo{
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

func (s *StarTracker) SaveCache(w io.Writer) error {
	return s.lru.Save(w)
}

func (s *StarTracker) LoadCache(r io.Reader) (err error) {
	s.lru, err = lru.Load(r, func(e json.RawMessage) (interface{}, error) {
		var r repo
		err := json.Unmarshal(e, &r)
		return &r, err
	}, s.lru.MaxEntries)
	return
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

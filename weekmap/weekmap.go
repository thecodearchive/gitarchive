package weekmap

import (
	"encoding/base64"
	"errors"
	"math/big"
	"time"
)

// WeekMap is a packed bitmap, where each bit represents one UTC hour in a week.
// Generate one from a ASCII representation with gen.go
type WeekMap big.Int

func Parse(packed string) (*WeekMap, error) {
	b, err := base64.StdEncoding.DecodeString(packed)
	if err != nil {
		return nil, err
	}
	if len(b) > 24*7/8 {
		return nil, errors.New("wrong length")
	}
	return (*WeekMap)(new(big.Int).SetBytes(b)), nil
}

func (w *WeekMap) Pack() string {
	i := (*big.Int)(w)
	return base64.StdEncoding.EncodeToString(i.Bytes())
}

func (w *WeekMap) Get(t time.Time) bool {
	pos := int(t.UTC().Weekday())*24 + t.UTC().Hour()
	i := (*big.Int)(w)
	return i.Bit(pos) == 1
}

func (w *WeekMap) Set(weekday time.Weekday, hour int, val bool) {
	pos := int(weekday)*24 + hour
	b := uint(0)
	if val {
		b = 1
	}
	i := (*big.Int)(w)
	i.SetBit(i, pos, b)
}

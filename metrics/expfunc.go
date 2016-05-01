package metrics

import "strconv"

type IntFunc func() int

func (f IntFunc) String() string {
	return strconv.Itoa(f())
}

func (f IntFunc) Int() int {
	return f()
}

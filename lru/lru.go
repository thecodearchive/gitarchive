/*
Copyright 2013 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package lru implements an LRU cache.
package lru

import (
	"container/list"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
)

// Cache is an LRU cache. It is not safe for concurrent access.
type Cache struct {
	// MaxEntries is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	MaxEntries int

	// OnEvicted optionally specificies a callback function to be
	// executed when an entry is purged from the cache.
	OnEvicted func(key string, value interface{})

	ll     *list.List
	cache  map[string]*list.Element
	length uint32
}

type entry struct {
	Key   string
	Value interface{}
}

// New creates a new Cache.
// If maxEntries is zero, the cache has no limit and it's assumed
// that eviction is done by the caller.
func New(maxEntries int) *Cache {
	return &Cache{
		MaxEntries: maxEntries,
		ll:         list.New(),
		cache:      make(map[string]*list.Element),
	}
}

func Load(r io.Reader, load func(json.RawMessage) (interface{}, error),
	maxEntries int) (*Cache, error) {
	c := New(maxEntries)
	dec := json.NewDecoder(r)
	loadedEntries := 0
	for {
		var e struct {
			Key   string
			Value json.RawMessage
		}
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		loadedEntries += 1
		if maxEntries != 0 && loadedEntries > maxEntries {
			return nil, errors.New("trying to load more than maxEntries")
		}
		value, err := load(e.Value)
		if err != nil {
			return nil, err
		}
		ele := c.ll.PushBack(&entry{e.Key, value})
		c.cache[e.Key] = ele
	}
	atomic.StoreUint32(&c.length, uint32(loadedEntries))
	return c, nil
}

// Add adds a value to the cache.
func (c *Cache) Add(key string, value interface{}) {
	if c.cache == nil {
		c.cache = make(map[string]*list.Element)
		c.ll = list.New()
	}
	if ee, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ee)
		ee.Value.(*entry).Value = value
		return
	}
	ele := c.ll.PushFront(&entry{key, value})
	c.cache[key] = ele
	atomic.AddUint32(&c.length, 1)
	if c.MaxEntries != 0 && c.ll.Len() > c.MaxEntries {
		c.RemoveOldest()
	}
}

// Get looks up a key's value from the cache.
func (c *Cache) Get(key string) (value interface{}, ok bool) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).Value, true
	}
	return
}

// Remove removes the provided key from the cache.
func (c *Cache) Remove(key string) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		c.removeElement(ele)
	}
}

// RemoveOldest removes the oldest item from the cache.
func (c *Cache) RemoveOldest() {
	if c.cache == nil {
		return
	}
	ele := c.ll.Back()
	if ele != nil {
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry)
	delete(c.cache, kv.Key)
	atomic.AddUint32(&c.length, ^uint32(0)) // decrement c.length
	if c.OnEvicted != nil {
		c.OnEvicted(kv.Key, kv.Value)
	}
}

// Len returns the number of items in the cache. It is safe to use concurrently
// with any other operations.
func (c *Cache) Len() uint32 {
	if c.cache == nil {
		return 0
	}
	return atomic.LoadUint32(&c.length)
}

// Save serializes the cache. All values must support json.Marshal.
func (c *Cache) Save(w io.Writer) error {
	if c.cache == nil {
		return nil
	}
	enc := json.NewEncoder(w)
	for e := c.ll.Front(); e != nil; e = e.Next() {
		if err := enc.Encode(e.Value.(*entry)); err != nil {
			return err
		}
	}
	return nil
}

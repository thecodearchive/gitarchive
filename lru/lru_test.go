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

package lru

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

type simpleStruct struct {
	A int
	B string
}

type complexStruct struct {
	A int
	B simpleStruct
}

var getTests = []struct {
	name       string
	keyToAdd   string
	keyToGet   string
	expectedOk bool
}{
	{"string_hit", "myKey", "myKey", true},
	{"string_miss", "myKey", "nonsense", false},
}

func TestGet(t *testing.T) {
	for _, tt := range getTests {
		lru := New(0)
		lru.Add(tt.keyToAdd, 1234)
		val, ok := lru.Get(tt.keyToGet)
		if ok != tt.expectedOk {
			t.Fatalf("%s: cache hit = %v; want %v", tt.name, ok, !ok)
		} else if ok && val != 1234 {
			t.Fatalf("%s expected get to return 1234 but got %v", tt.name, val)
		}
	}
}

func TestRemove(t *testing.T) {
	lru := New(0)
	lru.Add("myKey", 1234)
	if val, ok := lru.Get("myKey"); !ok {
		t.Fatal("TestRemove returned no match")
	} else if val != 1234 {
		t.Fatalf("TestRemove failed.  Expected %d, got %v", 1234, val)
	}

	lru.Remove("myKey")
	if _, ok := lru.Get("myKey"); ok {
		t.Fatal("TestRemove returned a removed entry")
	}
}

func TestSaveLoad(t *testing.T) {
	lru := New(0)
	lru.Add("myKey", complexStruct{1, simpleStruct{2, "three"}})
	lru.Add("myKey2", complexStruct{4, simpleStruct{5, "six"}})
	lru.Add("myKey3", complexStruct{7, simpleStruct{8, "nine"}})
	b := &bytes.Buffer{}
	if err := lru.Save(b); err != nil {
		t.Fatal(err)
	}

	newLRU, err := Load(b, func(e json.RawMessage) (interface{}, error) {
		var c complexStruct
		err := json.Unmarshal(e, &c)
		return c, err
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lru, newLRU) {
		t.Fail()
	}
}

package bombardier

import (
	"errors"
	"reflect"
	"testing"
)

func TestErrorMapAdd(t *testing.T) {
	m := NewErrorMap()
	err := errors.New("Add")
	m.Add(err)
	if c := m.Get(err); c != 1 {
		t.Error(c)
	}
}

func TestErrorMapGet(t *testing.T) {
	m := NewErrorMap()
	err := errors.New("Get")
	if c := m.Get(err); c != 0 {
		t.Error(c)
	}
}

func TestByFrequency(t *testing.T) {
	m := NewErrorMap()
	a := errors.New("A")
	b := errors.New("B")
	c := errors.New("C")
	m.Add(a)
	m.Add(a)
	m.Add(b)
	m.Add(b)
	m.Add(b)
	m.Add(c)
	e := ErrorsByFrequency{
		{"B", 3},
		{"A", 2},
		{"C", 1},
	}
	if a := m.ByFrequency(); !reflect.DeepEqual(a, e) {
		t.Logf("Expected: %+v", e)
		t.Logf("Got: %+v", a)
		t.Fail()
	}
}

func TestErrorWithCountToStringConversion(t *testing.T) {
	ewc := ErrorWithCount{"A", 1}
	exp := "<A:1>"
	if act := ewc.String(); act != exp {
		t.Logf("Expected: %+v", exp)
		t.Logf("Got: %+v", act)
		t.Fail()
	}
}

func BenchmarkErrorMapAdd(b *testing.B) {
	m := NewErrorMap()
	err := errors.New("benchmark")
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Add(err)
		}
	})
}

func BenchmarkErrorMapGet(b *testing.B) {
	m := NewErrorMap()
	err := errors.New("benchmark")
	m.Add(err)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Get(err)
		}
	})
}

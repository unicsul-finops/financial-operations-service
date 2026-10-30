package config

import (
	"reflect"
	"testing"
)

func TestOrigensDoCORS(t *testing.T) {
	casos := []struct {
		valor    string
		esperado []string
	}{
		{"", []string{"*"}},
		{"  ", []string{"*"}},
		{"*", []string{"*"}},
		{"http://a.com, http://b.com:3000 ,,", []string{"http://a.com", "http://b.com:3000"}},
		{"off", nil},
		{"OFF", nil},
	}
	for _, c := range casos {
		if obtido := origensDoCORS(c.valor); !reflect.DeepEqual(obtido, c.esperado) {
			t.Errorf("%q: %v, esperava %v", c.valor, obtido, c.esperado)
		}
	}
}

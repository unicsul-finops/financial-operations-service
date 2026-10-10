package service

import "testing"

func TestIdentificarOperacao(t *testing.T) {
	casos := []struct {
		entrada string
		id      int64
		numero  string
	}{
		{"1", 1, ""},
		{" 42 ", 42, ""},
		{"9223372036854775807", 9223372036854775807, ""},
		{"OP-2026-0001", 0, "OP-2026-0001"},
		{" op-2026-0001 ", 0, "OP-2026-0001"},
		{"OP-2027-10000", 0, "OP-2027-10000"},
	}

	for _, c := range casos {
		id, numero, err := identificarOperacao(c.entrada)
		if err != nil || id != c.id || numero != c.numero {
			t.Errorf("identificarOperacao(%q) = %d %q %v, esperava %d %q", c.entrada, id, numero, err, c.id, c.numero)
		}
	}

	invalidos := []string{"", "abc", "0", "-1", "1.5", "9223372036854775808", "OP-2026-1", "OP-26-0001", "OP-2026-000A", "3f1c2b7e-8a4d-4c6e-9b1a-2d3e4f5a6b7c"}
	for _, entrada := range invalidos {
		if _, _, err := identificarOperacao(entrada); err == nil {
			t.Errorf("identificarOperacao(%q) deveria dar erro", entrada)
		}
	}
}

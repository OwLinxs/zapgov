package surveys

import "testing"

func TestParseNota(t *testing.T) {
	cases := []struct {
		buttonID, body string
		want           int
		ok             bool
	}{
		{"nota_5", "⭐⭐⭐⭐⭐ Excelente", 5, true}, // toque na lista
		{"nota_1", "⭐ Muito ruim", 1, true},
		{"", "3", 3, true},                       // digitou o número
		{"", " 4 ", 4, true},                     // com espaços
		{"", "⭐⭐⭐ Regular", 3, true},             // título sem ID
		{"", "obrigado", 0, false},               // texto comum → não consome
		{"", "10", 0, false},                     // fora da escala
		{"nota_9", "", 0, false},                 // id inválido
		{"outro_id", "oi", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseNota(tc.buttonID, tc.body)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseNota(%q,%q) = (%d,%v), quer (%d,%v)", tc.buttonID, tc.body, got, ok, tc.want, tc.ok)
		}
	}
}

package messaging

import "testing"

func TestSetorEfetivo(t *testing.T) {
	cases := []struct {
		nome                             string
		papel, setorUsuario, setorPedido string
		rotear                           bool
		want                             string
	}{
		// Roteamento ligado: atendente é preso ao próprio setor.
		{"atendente forçado ao setor", "atendente", "Saúde", "", true, "Saúde"},
		{"atendente não burla pedindo outro setor", "atendente", "Saúde", "Tributos", true, "Saúde"},
		// Atendente sem setor definido: vê tudo (não trava acesso).
		{"atendente sem setor vê o que pedir", "atendente", "", "Tributos", true, "Tributos"},
		{"atendente sem setor vê tudo", "atendente", "", "", true, ""},
		// Roteamento desligado: filtro livre.
		{"roteamento off: atendente filtra livre", "atendente", "Saúde", "Tributos", false, "Tributos"},
		// Gestão nunca é forçada.
		{"admin filtra livre", "admin", "Saúde", "Tributos", true, "Tributos"},
		{"supervisor vê tudo", "supervisor", "Saúde", "", true, ""},
	}
	for _, tc := range cases {
		if got := setorEfetivo(tc.papel, tc.setorUsuario, tc.setorPedido, tc.rotear); got != tc.want {
			t.Errorf("%s: setorEfetivo(%q,%q,%q,%v) = %q, quer %q",
				tc.nome, tc.papel, tc.setorUsuario, tc.setorPedido, tc.rotear, got, tc.want)
		}
	}
}

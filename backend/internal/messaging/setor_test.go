package messaging

import (
	"strings"
	"testing"
)

func TestSetoresPermitidos(t *testing.T) {
	central := [][]string{{"Agendamento", "Saúde Mental", "MAC", "Recepção"}}
	cases := []struct {
		nome         string
		papel, setor string
		rotear       bool
		grupos       [][]string
		want         string // setores permitidos juntados por "," ; "<tudo>" = nil
	}{
		// Roteamento ligado, sem grupo: atendente isolado no próprio setor.
		{"atendente isolado", "atendente", "Unidade A", true, central, "Unidade A"},
		// Atendente num setor de um grupo: vê todos os setores do grupo.
		{"atendente vê o grupo", "atendente", "MAC", true, central, "Agendamento,Saúde Mental,MAC,Recepção"},
		// Atendente sem setor não recebe acesso implícito.
		{"atendente sem setor", "atendente", "", true, central, ""},
		// Roteamento desligado: vê tudo.
		{"roteamento off", "atendente", "Unidade A", false, central, "<tudo>"},
		// Gestão nunca é restringida.
		{"admin vê tudo", "admin", "MAC", true, central, "<tudo>"},
		{"supervisor vê tudo", "supervisor", "Unidade A", true, central, "<tudo>"},
	}
	for _, tc := range cases {
		got := setoresPermitidos(tc.papel, tc.setor, tc.rotear, tc.grupos)
		var s string
		if got == nil {
			s = "<tudo>"
		} else {
			s = strings.Join(got, ",")
		}
		if s != tc.want {
			t.Errorf("%s: setoresPermitidos(%q,%q,%v) = %q, quer %q", tc.nome, tc.papel, tc.setor, tc.rotear, s, tc.want)
		}
	}
}

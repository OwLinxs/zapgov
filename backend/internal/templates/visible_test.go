package templates

import "testing"

func TestVisibleFor(t *testing.T) {
	tpls := []Template{
		{Name: "saude_aviso", Setores: []string{"Saúde"}},
		{Name: "tributos_2via", Setores: []string{"Tributos"}},
		{Name: "geral_multi", Setores: []string{"Saúde", "Tributos"}},
		{Name: "sem_setor", Setores: []string{}}, // não atribuído
	}

	nomes := func(ts []Template) []string {
		out := make([]string, len(ts))
		for i, x := range ts {
			out[i] = x.Name
		}
		return out
	}
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	// Admin e supervisor veem tudo, com ou sem setor.
	for _, papel := range []string{"admin", "supervisor"} {
		if got := VisibleFor(tpls, papel, ""); len(got) != 4 {
			t.Errorf("%s deveria ver 4 templates, viu %d", papel, len(got))
		}
	}

	// Atendente da Saúde: só os do setor (sem_setor fica invisível).
	got := nomes(VisibleFor(tpls, "atendente", "Saúde"))
	if !eq(got, []string{"saude_aviso", "geral_multi"}) {
		t.Errorf("atendente Saúde viu %v", got)
	}

	// Atendente de Tributos.
	got = nomes(VisibleFor(tpls, "atendente", "Tributos"))
	if !eq(got, []string{"tributos_2via", "geral_multi"}) {
		t.Errorf("atendente Tributos viu %v", got)
	}

	// Atendente SEM setor: nenhum (política: sem atribuição = só gestão usa).
	if got := VisibleFor(tpls, "atendente", ""); len(got) != 0 {
		t.Errorf("atendente sem setor deveria ver 0, viu %d", len(got))
	}

	// Setor que não bate com nada.
	if got := VisibleFor(tpls, "atendente", "Obras"); len(got) != 0 {
		t.Errorf("setor sem templates deveria ver 0, viu %d", len(got))
	}
}

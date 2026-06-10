package campaigns

import "testing"

func TestRender(t *testing.T) {
	body := "Olá {{1}}, sua consulta é dia {{2}} às {{3}}."
	got := render(body, []string{"Maria", "30/06/2026", "14h"})
	want := "Olá Maria, sua consulta é dia 30/06/2026 às 14h."
	if got != want {
		t.Errorf("render = %q, quer %q", got, want)
	}
	// Param faltando → mantém o placeholder (visível no histórico).
	got = render(body, []string{"João"})
	if got != "Olá João, sua consulta é dia {{2}} às {{3}}." {
		t.Errorf("render parcial = %q", got)
	}
}

func TestAcharDataCampanha(t *testing.T) {
	if d, ok := acharData([]string{"Maria", "30/06/2026"}); !ok || d.Day() != 30 {
		t.Errorf("não achou a data (ok=%v d=%v)", ok, d)
	}
	if _, ok := acharData([]string{"sem data"}); ok {
		t.Error("achou data inexistente")
	}
}

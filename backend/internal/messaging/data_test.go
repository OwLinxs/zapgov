package messaging

import "testing"

func TestAcharData(t *testing.T) {
	// Detecta a data no meio das variáveis.
	if d, ok := acharData([]string{"Maria", "30/06/2026", "Posto Central"}); !ok || d.Format("02/01/2006") != "30/06/2026" {
		t.Errorf("não detectou 30/06/2026 (ok=%v, d=%v)", ok, d)
	}
	// Data dentro de um texto maior.
	if d, ok := acharData([]string{"sua consulta é dia 05/12/2026 às 14h"}); !ok || d.Format("02/01/2006") != "05/12/2026" {
		t.Errorf("não detectou data embutida (ok=%v, d=%v)", ok, d)
	}
	// Sem data → não dispara.
	if _, ok := acharData([]string{"Maria", "Posto Central", "14h30"}); ok {
		t.Error("detectou data onde não há")
	}
	// Data inválida (mês 99) → ignorada.
	if _, ok := acharData([]string{"05/99/2026"}); ok {
		t.Error("aceitou data inválida")
	}
}

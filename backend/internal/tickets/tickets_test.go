package tickets

import (
	"testing"
	"time"
)

func TestTransicaoValida(t *testing.T) {
	cases := []struct {
		de, para string
		ok       bool
	}{
		{"aberto", "em_andamento", true},
		{"aberto", "aguardando", true},
		{"aberto", "cancelado", true},
		{"aberto", "resolvido", false}, // não pula direto p/ resolvido
		{"em_andamento", "resolvido", true},
		{"em_andamento", "aguardando", true},
		{"aguardando", "em_andamento", true},
		{"resolvido", "em_andamento", true},  // reabrir
		{"resolvido", "aberto", false},       // não volta a aberto
		{"cancelado", "em_andamento", false}, // cancelado é final
		{"aberto", "aberto", true},           // mesmo status é no-op válido
	}
	for _, c := range cases {
		if got := transicaoValida(c.de, c.para); got != c.ok {
			t.Errorf("transicaoValida(%q,%q) = %v, esperado %v", c.de, c.para, got, c.ok)
		}
	}
}

func TestSLAVenc(t *testing.T) {
	base := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	horas := map[string]int{"critica": 4, "alta": 8, "media": 24, "baixa": 72}

	got := slaVenc(base, "critica", horas)
	if want := base.Add(4 * time.Hour); !got.Equal(want) {
		t.Errorf("crítica: got %v, want %v", got, want)
	}
	// prioridade sem config cai no padrão (média = 24h).
	got = slaVenc(base, "media", map[string]int{})
	if want := base.Add(24 * time.Hour); !got.Equal(want) {
		t.Errorf("padrão média: got %v, want %v", got, want)
	}
	// prioridade desconhecida → fallback 24h.
	got = slaVenc(base, "xxx", map[string]int{})
	if want := base.Add(24 * time.Hour); !got.Equal(want) {
		t.Errorf("desconhecida: got %v, want %v", got, want)
	}
}

func TestPrioridadeValida(t *testing.T) {
	for _, p := range []string{"baixa", "media", "alta", "critica"} {
		if !prioridadeValida(p) {
			t.Errorf("%q deveria ser válida", p)
		}
	}
	for _, p := range []string{"", "urgente", "Media"} {
		if prioridadeValida(p) {
			t.Errorf("%q não deveria ser válida", p)
		}
	}
}

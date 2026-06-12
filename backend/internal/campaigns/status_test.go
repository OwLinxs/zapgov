package campaigns

import "testing"

func TestStatusDeltas(t *testing.T) {
	cases := []struct {
		old, neu           string
		ent, lidas, falhas int
	}{
		{"sent", "delivered", 1, 0, 0},
		{"sent", "read", 1, 1, 0}, // read sem delivered antes: conta os dois
		{"delivered", "read", 0, 1, 0},
		{"sent", "failed", 0, 0, 1},
		{"delivered", "delivered", 0, 0, 0}, // reentrega do webhook não duplica
		{"read", "delivered", 0, 0, 0},      // regressão ignorada
		{"read", "read", 0, 0, 0},
		{"failed", "failed", 0, 0, 0},
		{"pending", "delivered", 1, 0, 0},
	}
	for _, c := range cases {
		ent, lidas, falhas := statusDeltas(c.old, c.neu)
		if ent != c.ent || lidas != c.lidas || falhas != c.falhas {
			t.Errorf("statusDeltas(%q,%q) = (%d,%d,%d), esperado (%d,%d,%d)",
				c.old, c.neu, ent, lidas, falhas, c.ent, c.lidas, c.falhas)
		}
	}
}

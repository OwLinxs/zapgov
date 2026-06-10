package messaging

import "testing"

func TestCanonPhone(t *testing.T) {
	cases := map[string]string{
		// Com DDI: reinsere o 9º dígito quando falta.
		"554691393338":      "5546991393338",
		"5546991393338":     "5546991393338",
		"+55 46 99139-3338": "5546991393338",
		"551133334444":      "551133334444", // fixo: inalterado

		// SEM DDI (o bug das conversas duplicadas): prefixa 55.
		"46991393338":     "5546991393338", // celular 11 díg
		"4691393338":      "5546991393338", // celular 10 díg (sem o 9) → 55 + reinsere
		"(46) 99139-3338": "5546991393338", // com máscara, sem DDI

		// Internacionais: NÃO ganham 55.
		"12025550100":  "12025550100",  // EUA
		"447911123456": "447911123456", // UK
	}
	for in, want := range cases {
		if got := CanonPhone(in); got != want {
			t.Errorf("CanonPhone(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestIsStopWord(t *testing.T) {
	for _, w := range []string{"SAIR", "PARAR", "CANCELAR", "STOP", "DESCADASTRAR", "REMOVER"} {
		if !isStopWord(w) {
			t.Errorf("isStopWord(%q) deveria ser true", w)
		}
	}
	for _, w := range []string{"OI", "VOLTAR", "SAIRDAQUI", "", "1"} {
		if isStopWord(w) {
			t.Errorf("isStopWord(%q) deveria ser false", w)
		}
	}
}

func TestMediaKind(t *testing.T) {
	cases := map[string]string{
		"image/jpeg":      "image",
		"image/png":       "image",
		"audio/ogg":       "audio",
		"video/mp4":       "video",
		"application/pdf": "document",
		"text/plain":      "document",
		"":                "document", // sem mime → documento (mais seguro)
	}
	for mime, want := range cases {
		if got := mediaKind(mime); got != want {
			t.Errorf("mediaKind(%q) = %q, quer %q", mime, got, want)
		}
	}
}

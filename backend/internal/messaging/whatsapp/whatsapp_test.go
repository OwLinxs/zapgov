package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"554691393338":        "5546991393338", // BR celular sem o 9 → reinsere
		"5546991393338":       "5546991393338", // já correto
		"+55 (46) 99139-3338": "5546991393338", // com máscara
		"551133334444":        "551133334444",  // fixo SP (8 díg começando 3) → inalterado
		"5511999998888":       "5511999998888", // celular já com 9
		"12025550100":         "12025550100",   // não-BR → só dígitos
	}
	for in, want := range cases {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestVerifySignature(t *testing.T) {
	p := New("", "tok", "segredo-do-app", "v21.0")
	body := []byte(`{"object":"whatsapp","entry":[]}`)

	mac := hmac.New(sha256.New, []byte("segredo-do-app"))
	mac.Write(body)
	valid := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !p.VerifySignature(body, valid) {
		t.Error("assinatura válida foi rejeitada")
	}
	if p.VerifySignature(body, "sha256=deadbeef") {
		t.Error("assinatura inválida foi aceita")
	}
	if p.VerifySignature(body, "") {
		t.Error("assinatura vazia foi aceita")
	}
}

func TestParseWebhook(t *testing.T) {
	body := []byte(`{
	  "entry":[{"changes":[{"value":{
	    "contacts":[{"profile":{"name":"João"},"wa_id":"5511999998888"}],
	    "messages":[
	      {"from":"5511999998888","id":"wamid.1","timestamp":"1700000000","type":"text","text":{"body":"oi"}},
	      {"from":"5511999998888","id":"wamid.2","timestamp":"1700000001","type":"image","image":{"id":"media1","mime_type":"image/jpeg","caption":"foto"}}
	    ],
	    "statuses":[{"id":"wamid.0","status":"read","timestamp":"1700000002","recipient_id":"5511999998888"}]
	  }}]}]
	}`)

	p := New("", "", "", "v21.0")
	incoming, statuses, err := p.ParseWebhook(body)
	if err != nil {
		t.Fatalf("ParseWebhook erro: %v", err)
	}
	if len(incoming) != 2 {
		t.Fatalf("esperava 2 inbound, veio %d", len(incoming))
	}
	if incoming[0].Body != "oi" || incoming[0].Name != "João" {
		t.Errorf("texto inesperado: %+v", incoming[0])
	}
	if incoming[1].MediaType != "image" || incoming[1].MediaID != "media1" || incoming[1].Body != "foto" {
		t.Errorf("mídia inesperada: %+v", incoming[1])
	}
	if len(statuses) != 1 || statuses[0].Status != "read" {
		t.Errorf("status inesperado: %+v", statuses)
	}
}

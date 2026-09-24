package flow

import (
	"encoding/json"
	"strings"
	"testing"

	"zapgov/backend/internal/tenants"
)

func TestFlowCredentialsLifecycle(t *testing.T) {
	t.Setenv("META_CREDENTIALS_KEY", strings.Repeat("ab", 32))
	graph := func(base, key string) json.RawMessage {
		d, _ := json.Marshal(map[string]string{"integration": "sige_ti", "base_url": base, "api_key": key, "body": `{"defeito_relatado":"teste"}`})
		raw, _ := json.Marshal(Graph{Nodes: []Node{{ID: "api1", Type: "api", Data: d}}})
		return raw
	}
	empty := json.RawMessage(`{"nodes":[],"edges":[]}`)
	saved, err := secureGraph(graph("https://sige.example", "secret-key"), empty, 7)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "secret-key") {
		t.Fatal("plaintext saved")
	}
	var g Graph
	_ = json.Unmarshal(saved, &g)
	var d apiData
	_ = json.Unmarshal(g.Nodes[0].Data, &d)
	plain, err := tenants.OpenIntegrationSecret(d.EncryptedKey, credentialBinding(7, d.BaseURL))
	if err != nil || plain != "secret-key" {
		t.Fatal("cannot decrypt saved key")
	}
	if _, err := tenants.OpenIntegrationSecret(d.EncryptedKey, credentialBinding(8, d.BaseURL)); err == nil {
		t.Fatal("cross-tenant decrypt")
	}
	public := publicGraph(saved)
	if strings.Contains(string(public), "enc:v1:") || strings.Contains(string(public), "secret-key") || !strings.Contains(string(public), `"api_key_configured":true`) {
		t.Fatalf("bad redaction: %s", public)
	}
	kept, err := secureGraph(public, saved, 7)
	if err != nil || !strings.Contains(string(kept), d.EncryptedKey) {
		t.Fatal("blank key did not preserve stored credential")
	}
	if _, err := secureGraph(graph("https://another.example", ""), saved, 7); err == nil {
		t.Fatal("destination changed without new credential")
	}
	replaced, err := secureGraph(graph("https://another.example", "new-key"), saved, 7)
	if err != nil || strings.Contains(string(replaced), d.EncryptedKey) {
		t.Fatal("key rotation failed")
	}
	// A browser-supplied ciphertext cannot create a credential or import one.
	if _, err := secureGraph(saved, empty, 8); err == nil {
		t.Fatal("accepted injected ciphertext")
	}
	removed, err := secureGraph(empty, saved, 7)
	if err != nil || strings.Contains(string(removed), "enc:v1:") {
		t.Fatal("deletion retained credential")
	}
}

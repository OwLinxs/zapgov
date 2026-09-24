package flow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"zapgov/backend/internal/tenants"

	"zapgov/backend/internal/tenantctx"
)

func TestSIGETIContract(t *testing.T) {
	for _, code := range []int{200, 201, 401, 422, 503, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/integracao/ordens-servico" || r.Header.Get("X-API-Key") != "test-secret" {
					t.Error("wrong request")
				}
				var body map[string]string
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("invalid JSON")
				}
				if body["defeito_relatado"] != "Internet \"caiu\"\n{{nome}}" || body["referencia_externa"] != "wa-test" || body["origem"] != "whatsapp" {
					t.Errorf("wrong payload: %v", body)
				}
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"id":42,"numero":"OS-2026-0042","status":"aberta"}`))
			}))
			defer server.Close()

			d := apiData{Integration: "sige_ti", Body: `{"defeito_relatado":"{{defeito}}","prioridade":"alta"}`, Response: map[string]string{"numero": "numero", "id": "id", "status": "status"}}
			raw, _ := json.Marshal(d)
			raw = configuredAPI(t, raw, server.URL, "test-secret", 7)
			vars := map[string]string{"defeito": "Internet \"caiu\"\n{{nome}}", "_referencia_externa": "wa-test", "numero": "stale"}
			got := callAPI(tenantctx.With(context.Background(), 7), raw, vars)
			if code == 200 || code == 201 {
				if got != "success" || vars["numero"] != "OS-2026-0042" || vars["id"] != "42" || vars["status"] != "aberta" {
					t.Fatalf("%s %v", got, vars)
				}
				// Replay preserves the caller's persisted reference.
				if callAPI(tenantctx.With(context.Background(), 7), raw, vars) != "success" {
					t.Fatal("replay failed")
				}
			} else if got != "error" || vars["api_erro"] == "" || vars["numero"] != "" {
				t.Fatalf("%s %v", got, vars)
			}
			before := calls
			if callAPI(tenantctx.With(context.Background(), 8), raw, vars) != "error" || calls != before {
				t.Fatal("cross-tenant request")
			}
		})
	}
}

func TestSIGETIRejectsRedirectAndInvalidResponse(t *testing.T) {
	for _, body := range []string{"redirect", `not json`, `{"id":42}`, `{"id":0,"numero":"OS","status":"aberta"}`} {
		t.Run(body, func(t *testing.T) {
			leaked := false
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if body == "redirect" {
					http.Redirect(w, r, target.URL, 302)
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			raw := json.RawMessage(`{"integration":"sige_ti","body":"{\"defeito_relatado\":\"test\"}"}`)
			raw = configuredAPI(t, raw, server.URL, "secret", 1)
			vars := map[string]string{"_referencia_externa": "wa-test"}
			if callAPI(tenantctx.With(context.Background(), 1), raw, vars) != "error" || leaked {
				t.Fatal("accepted invalid response or leaked key")
			}
		})
	}
}

func TestAPIPayloadRejectsMissingVariablesAndInvalidPriority(t *testing.T) {
	for _, body := range []string{`{"defeito_relatado":"{{missing}}"}`, `{"defeito_relatado":""}`, `{"defeito_relatado":"x","prioridade":"critica"}`, `[]`} {
		if _, err := apiPayload(apiData{Body: body}, map[string]string{"_referencia_externa": "wa-test"}); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestParseAPIValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body, target string
		valid              bool
	}{
		{"valid", `{"defeito_relatado":"{{defeito}}"}`, "numero", true},
		{"missing defect", `{}`, "numero", false},
		{"invalid JSON", `{`, "numero", false},
		{"reserved variable", `{"defeito_relatado":"x"}`, "_referencia_externa", false},
		{"invalid variable", `{"defeito_relatado":"x"}`, "bad name", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(apiData{Integration: "sige_ti", Body: tc.body, Response: map[string]string{"numero": tc.target}})
			raw, _ := json.Marshal(Graph{Nodes: []Node{{ID: "api", Type: "api", Data: data}}})
			_, err := Parse(raw)
			if (err == nil) != tc.valid {
				t.Fatalf("Parse error: %v", err)
			}
		})
	}
}

func TestRenderDoesNotExpandCapturedContent(t *testing.T) {
	if got := render("Olá {{nome}}", map[string]string{"nome": "{{segredo}}", "segredo": "valor"}); got != "Olá {{segredo}}" {
		t.Fatalf("nested interpolation: %s", got)
	}
}

func TestSIGETICanceledRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("canceled request was sent") }))
	defer server.Close()

	ctx, cancel := context.WithCancel(tenantctx.With(context.Background(), 1))
	cancel()
	vars := map[string]string{"_referencia_externa": "wa-test"}
	raw := json.RawMessage(`{"integration":"sige_ti","body":"{\"defeito_relatado\":\"test\"}"}`)
	raw = configuredAPI(t, raw, server.URL, "secret", 1)
	if callAPI(ctx, raw, vars) != "error" || vars["api_erro"] != "resultado_incerto" {
		t.Fatalf("wrong result: %v", vars)
	}
}

func configuredAPI(t *testing.T, raw json.RawMessage, base, key string, tid int64) json.RawMessage {
	t.Helper()
	t.Setenv("META_CREDENTIALS_KEY", strings.Repeat("ab", 32))
	var d apiData
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	d.BaseURL = base
	var err error
	d.EncryptedKey, err = tenants.SealIntegrationSecret(key, credentialBinding(tid, base))
	if err != nil {
		t.Fatal(err)
	}
	result, _ := json.Marshal(d)
	return result
}

package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func assinaturaValida(secret string, body []byte, header string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return header == "sha256="+hex.EncodeToString(mac.Sum(nil))
}

func TestAbrirChamadoAssinaEEnvia(t *testing.T) {
	const key, secret = "sige-key", "sige-hmac"
	var gotEvento string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/integracao/zapgov/eventos" {
			w.WriteHeader(404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("X-API-Key") != key {
			t.Errorf("api key errada: %q", r.Header.Get("X-API-Key"))
		}
		if !assinaturaValida(secret, body, r.Header.Get("X-Signature")) {
			t.Errorf("assinatura inválida: %q", r.Header.Get("X-Signature"))
		}
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		gotEvento, _ = m["evento"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{"ticket_id": 55, "numero": "OS-2026-0055"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, key, secret)
	resp, err := c.AbrirChamado(context.Background(), "5546999990000", "João", "wa-abc",
		map[string]string{"descricao": "pc não liga"}, Janela{Aberta: true})
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	if resp.TicketID != 55 || resp.Numero != "OS-2026-0055" {
		t.Fatalf("resposta errada: %+v", resp)
	}
	if gotEvento != "chamado.abrir" {
		t.Fatalf("evento errado: %q", gotEvento)
	}
}

func TestListarChamados(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chamados" || r.Header.Get("X-API-Key") != "k" {
			w.WriteHeader(401)
			return
		}
		if r.URL.Query().Get("telefone") != "5546999990000" {
			t.Errorf("telefone não repassado: %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"chamados": []map[string]any{
			{"id": 12, "numero": "OS-0012", "resumo": "Impressora", "status": "aberto", "fechado": false},
			{"id": 9, "numero": "OS-0009", "resumo": "Rede", "status": "resolvido", "fechado": true},
		}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k", "s")
	list, err := c.ListarChamados(context.Background(), "5546999990000", 10)
	if err != nil {
		t.Fatalf("listar: %v", err)
	}
	if len(list) != 2 || list[0].ID != 12 || !list[1].Fechado {
		t.Fatalf("lista errada: %+v", list)
	}
}

func TestAbrirChamadoErroHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k", "s")
	if _, err := c.AbrirChamado(context.Background(), "55", "n", "wa-x", nil, Janela{}); err == nil {
		t.Fatal("esperava erro em 503")
	}
}

package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAssinadorRoundTripEAdulteracao(t *testing.T) {
	a := NovoAssinador("k", "https://z", time.Minute)
	u := a.URL(MidiaRef{TenantID: 2, MediaID: "m", Mime: "image/png"})
	tok := strings.TrimPrefix(u, "https://z/v1/midia/")
	if ref, err := a.Validar(tok); err != nil || ref.MediaID != "m" || ref.TenantID != 2 {
		t.Fatalf("roundtrip: %+v %v", ref, err)
	}
	// outra chave não valida
	if _, err := NovoAssinador("outra", "https://z", time.Minute).Validar(tok); err == nil {
		t.Fatal("token aceito com outra chave")
	}
	// corpo trocado não valida
	corpo, sig, _ := strings.Cut(tok, ".")
	if _, err := a.Validar(corpo + "x." + sig); err == nil {
		t.Fatal("token adulterado aceito")
	}
}

func TestAssinadorExpira(t *testing.T) {
	a := NovoAssinador("k", "https://z", time.Minute)
	tok := a.token(MidiaRef{TenantID: 2, MediaID: "m", Expira: time.Now().Add(-time.Second).Unix()})
	if _, err := a.Validar(tok); err == nil {
		t.Fatal("token expirado aceito")
	}
}

func TestAssinadorSemURLPublica(t *testing.T) {
	if u := NovoAssinador("k", "", time.Minute).URL(MidiaRef{TenantID: 2, MediaID: "m"}); u != "" {
		t.Fatalf("sem GATEWAY_PUBLIC_URL deveria vir vazio: %q", u)
	}
}

func TestBaixadorSoDoSige(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		switch r.URL.Path {
		case "/arq/laudo.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF"))
		case "/redir":
			http.Redirect(w, r, "http://169.254.169.254/", http.StatusFound)
		case "/grande.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(make([]byte, 5<<20+1))
		}
	}))
	defer srv.Close()
	b := NovoBaixador(srv.URL+"/", "chave-sige")
	ctx := context.Background()

	arq, err := b.Baixar(ctx, srv.URL+"/arq/laudo.pdf", "", "")
	if err != nil || arq.Mime != "application/pdf" || arq.Nome != "laudo.pdf" || string(arq.Dados) != "%PDF" || gotKey != "chave-sige" {
		t.Fatalf("download: %+v %v key=%q", arq, err, gotKey)
	}
	for _, u := range []string{"http://outro.host/x.pdf", "ftp://" + strings.TrimPrefix(srv.URL, "http://") + "/x", "::lixo"} {
		if _, err := b.Baixar(ctx, u, "", ""); !errors.Is(err, errMidiaOrigem) {
			t.Fatalf("%s: deveria recusar origem: %v", u, err)
		}
	}
	if _, err := b.Baixar(ctx, srv.URL+"/redir", "", ""); !errors.Is(err, errMidiaBaixada) {
		t.Fatalf("redirect não pode ser seguido: %v", err)
	}
	if _, err := b.Baixar(ctx, srv.URL+"/grande.jpg", "", ""); !errors.Is(err, errMidiaGrande) {
		t.Fatalf("imagem > 5MB deveria ser recusada: %v", err)
	}
}

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"zapgov/backend/internal/messaging"
)

type fakeSender struct {
	err      error
	wa       string
	upload   *[]string // kind|filename|mime|caption de SendMediaUpload
	media    []byte
	mediaMim string
}

func (s fakeSender) SendText(ctx context.Context, to, body string) (*messaging.Message, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &messaging.Message{WAMessageID: s.wa}, nil
}
func (s fakeSender) SendTemplate(ctx context.Context, to, name, lang string, params []string, preview string) (*messaging.Message, error) {
	return &messaging.Message{WAMessageID: s.wa}, s.err
}
func (s fakeSender) SendMediaUpload(ctx context.Context, to, kind string, data []byte, filename, mime, caption string) (*messaging.Message, error) {
	if s.upload != nil {
		*s.upload = append(*s.upload, kind+"|"+filename+"|"+mime+"|"+caption+"|"+string(data))
	}
	return &messaging.Message{WAMessageID: s.wa}, s.err
}
func (s fakeSender) DownloadMedia(ctx context.Context, mediaID string) ([]byte, string, error) {
	if s.media == nil {
		return nil, "", errors.New("sem mídia")
	}
	return s.media, s.mediaMim, nil
}

type fakeBaixador struct {
	arq *Arquivo
	err error
}

func (b fakeBaixador) Baixar(ctx context.Context, rawURL, mime, nome string) (*Arquivo, error) {
	return b.arq, b.err
}

var testAssinador = NovoAssinador("hmac-teste", "https://zapgov.exemplo", time.Minute)

type fakeCloser struct{ cleared, touched []string }

func (c *fakeCloser) Touch(ctx context.Context, phone string) error {
	c.touched = append(c.touched, phone)
	return nil
}

func (c *fakeCloser) Clear(ctx context.Context, phone string) error {
	c.cleared = append(c.cleared, phone)
	return nil
}

func appWith(sender Sender, closer relayCloser) *fiber.App {
	return appWithBaixador(sender, closer, fakeBaixador{})
}

func appWithBaixador(sender Sender, closer relayCloser, b baixador) *fiber.App {
	app := fiber.New()
	NewHandler(closer, sender, 2, "segredo", testAssinador, b).Register(app)
	return app
}

func get(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", path, nil))
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	return resp
}

func post(t *testing.T, app *fiber.App, path, key, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m
}

func TestEnviarSemChave401(t *testing.T) {
	app := appWith(fakeSender{wa: "wamid.1"}, &fakeCloser{})
	code, _ := post(t, app, "/v1/mensagens", "", `{"telefone":"55","tipo":"texto","texto":"oi"}`)
	if code != 401 {
		t.Fatalf("esperava 401, veio %d", code)
	}
}

func TestEnviarTextoOK(t *testing.T) {
	closer := &fakeCloser{}
	app := appWith(fakeSender{wa: "wamid.99"}, closer)
	code, body := post(t, app, "/v1/mensagens", "segredo", `{"telefone":"55","tipo":"texto","texto":"oi"}`)
	if code != 200 || body["wa_message_id"] != "wamid.99" {
		t.Fatalf("esperava 200 com wa id, veio %d %v", code, body)
	}
	if len(closer.touched) != 1 || closer.touched[0] != "55" {
		t.Fatalf("resposta do técnico deveria marcar movimento: %+v", closer.touched)
	}
}

func TestEnviarJanelaFechada(t *testing.T) {
	app := appWith(fakeSender{err: messaging.ErrWindowClosed}, &fakeCloser{})
	code, body := post(t, app, "/v1/mensagens", "segredo", `{"telefone":"55","tipo":"texto","texto":"oi"}`)
	if code != 422 || body["erro"] != "janela_fechada" {
		t.Fatalf("esperava 422 janela_fechada, veio %d %v", code, body)
	}
}

func TestEncerrarRelay(t *testing.T) {
	closer := &fakeCloser{}
	app := appWith(fakeSender{wa: "x"}, closer)
	code, body := post(t, app, "/v1/relay/encerrar", "segredo", `{"telefone":"5546999990000"}`)
	if code != 200 || body["ok"] != true {
		t.Fatalf("esperava 200 ok, veio %d %v", code, body)
	}
	if len(closer.cleared) != 1 || closer.cleared[0] != "5546999990000" {
		t.Fatalf("deveria limpar o telefone: %+v", closer.cleared)
	}
}

func tokenDe(u string) string { return strings.TrimPrefix(u, "https://zapgov.exemplo") }

func TestMidiaRecebidaSemChaveServeComToken(t *testing.T) {
	app := appWith(fakeSender{media: []byte("JPEG"), mediaMim: "image/jpeg"}, &fakeCloser{})
	path := tokenDe(testAssinador.URL(MidiaRef{TenantID: 2, MediaID: "m1", Nome: "foto.jpg"}))
	resp := get(t, app, path)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "JPEG" || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("esperava a imagem: %d %q %q", resp.StatusCode, body, resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("faltou nosniff")
	}
}

func TestMidiaRecebidaHTMLViraDownload(t *testing.T) {
	app := appWith(fakeSender{media: []byte("<script>"), mediaMim: "text/html"}, &fakeCloser{})
	path := tokenDe(testAssinador.URL(MidiaRef{TenantID: 2, MediaID: "m1", Nome: "x\".html"}))
	resp := get(t, app, path)
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("html não pode ir inline: %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="x.html"` {
		t.Fatalf("disposition errado: %q", cd)
	}
}

func TestMidiaRecebidaTokenInvalido404(t *testing.T) {
	app := appWith(fakeSender{media: []byte("x"), mediaMim: "image/png"}, &fakeCloser{})
	outro := tokenDe(testAssinador.URL(MidiaRef{TenantID: 9, MediaID: "m1"})) // tenant errado
	for _, p := range []string{"/v1/midia/lixo", "/v1/midia/abc.def", outro} {
		if resp := get(t, app, p); resp.StatusCode != 404 {
			t.Fatalf("%s: esperava 404, veio %d", p, resp.StatusCode)
		}
	}
}

func TestEnviarMidiaBaixaDoSigeESobe(t *testing.T) {
	var ups []string
	b := fakeBaixador{arq: &Arquivo{Dados: []byte("PDF"), Mime: "application/pdf", Nome: "laudo.pdf"}}
	app := appWithBaixador(fakeSender{wa: "wamid.M", upload: &ups}, &fakeCloser{}, b)
	code, body := post(t, app, "/v1/mensagens", "segredo",
		`{"telefone":"55","tipo":"midia","midia_url":"https://sige/x.pdf","texto":"segue"}`)
	if code != 200 || body["wa_message_id"] != "wamid.M" {
		t.Fatalf("esperava 200: %d %v", code, body)
	}
	if len(ups) != 1 || ups[0] != "document|laudo.pdf|application/pdf|segue|PDF" {
		t.Fatalf("upload errado: %+v", ups)
	}
}

func TestEnviarMidiaForaDoSige422(t *testing.T) {
	app := appWithBaixador(fakeSender{wa: "x"}, &fakeCloser{}, fakeBaixador{err: errMidiaOrigem})
	code, body := post(t, app, "/v1/mensagens", "segredo",
		`{"telefone":"55","tipo":"midia","midia_url":"http://169.254.169.254/"}`)
	if code != 422 || body["erro"] != "midia_invalida" {
		t.Fatalf("esperava 422 midia_invalida: %d %v", code, body)
	}
}

func TestEnviarMidiaSigeFora502(t *testing.T) {
	app := appWithBaixador(fakeSender{wa: "x"}, &fakeCloser{}, fakeBaixador{err: errMidiaBaixada})
	code, body := post(t, app, "/v1/mensagens", "segredo",
		`{"telefone":"55","tipo":"midia","midia_url":"https://sige/x"}`)
	if code != 502 || body["erro"] != "midia_indisponivel" {
		t.Fatalf("esperava 502 midia_indisponivel: %d %v", code, body)
	}
}

package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"zapgov/backend/internal/messaging"
)

type fakeSender struct {
	err error
	wa  string
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
func (s fakeSender) SendMedia(ctx context.Context, to, kind, link, caption, name string) (*messaging.Message, error) {
	return &messaging.Message{WAMessageID: s.wa}, s.err
}

type fakeCloser struct{ cleared []string }

func (c *fakeCloser) Clear(ctx context.Context, phone string) error {
	c.cleared = append(c.cleared, phone)
	return nil
}

func appWith(sender Sender, closer relayCloser) *fiber.App {
	app := fiber.New()
	NewHandler(closer, sender, 2, "segredo").Register(app)
	return app
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
	app := appWith(fakeSender{wa: "wamid.99"}, &fakeCloser{})
	code, body := post(t, app, "/v1/mensagens", "segredo", `{"telefone":"55","tipo":"texto","texto":"oi"}`)
	if code != 200 || body["wa_message_id"] != "wamid.99" {
		t.Fatalf("esperava 200 com wa id, veio %d %v", code, body)
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

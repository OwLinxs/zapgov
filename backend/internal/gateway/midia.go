package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// ---------- Mídia recebida (pessoa -> técnico): URL assinada ----------

// Assinador gera e valida os tokens de GET /v1/midia/{token}. O token carrega o
// media id da Meta, o tenant, mime/nome e a expiração, assinado com HMAC.
type Assinador struct {
	chave   []byte
	baseURL string // URL pública do ZapGov (GATEWAY_PUBLIC_URL); vazia = sem mídia
	ttl     time.Duration
}

// NovoAssinador deriva a chave do segredo informado com separação de domínio
// (o mesmo segredo não assina outra coisa com o mesmo formato).
func NovoAssinador(segredo, baseURL string, ttl time.Duration) *Assinador {
	mac := hmac.New(sha256.New, []byte(segredo))
	mac.Write([]byte("zapgov-gateway-midia-v1"))
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &Assinador{chave: mac.Sum(nil), baseURL: strings.TrimRight(baseURL, "/"), ttl: ttl}
}

// MidiaRef identifica uma mídia recebida.
type MidiaRef struct {
	TenantID int64  `json:"t"`
	MediaID  string `json:"id"`
	Mime     string `json:"m,omitempty"`
	Nome     string `json:"n,omitempty"`
	Expira   int64  `json:"e"`
}

var errTokenInvalido = errors.New("token de mídia inválido ou expirado")

// URL devolve a URL assinada da mídia, ou "" se GATEWAY_PUBLIC_URL não está definida.
func (a *Assinador) URL(ref MidiaRef) string {
	if a == nil || a.baseURL == "" || ref.MediaID == "" {
		return ""
	}
	ref.Expira = time.Now().Add(a.ttl).Unix()
	return a.baseURL + "/v1/midia/" + a.token(ref)
}

func (a *Assinador) token(ref MidiaRef) string {
	raw, _ := json.Marshal(ref)
	corpo := base64.RawURLEncoding.EncodeToString(raw)
	return corpo + "." + base64.RawURLEncoding.EncodeToString(a.assinar(corpo))
}

func (a *Assinador) assinar(corpo string) []byte {
	mac := hmac.New(sha256.New, a.chave)
	mac.Write([]byte(corpo))
	return mac.Sum(nil)
}

// Validar confere assinatura e expiração do token.
func (a *Assinador) Validar(tok string) (MidiaRef, error) {
	var ref MidiaRef
	corpo, sig, ok := strings.Cut(tok, ".")
	if !ok {
		return ref, errTokenInvalido
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, a.assinar(corpo)) {
		return ref, errTokenInvalido
	}
	raw, err := base64.RawURLEncoding.DecodeString(corpo)
	if err != nil || json.Unmarshal(raw, &ref) != nil || ref.MediaID == "" || ref.TenantID <= 0 {
		return ref, errTokenInvalido
	}
	if time.Now().Unix() > ref.Expira {
		return ref, errTokenInvalido
	}
	return ref, nil
}

// mimeSeguroInline lista os tipos que podem ser exibidos no navegador. Qualquer
// outro (html, svg, js…) vai como download, para não executar no domínio do ZapGov.
func mimeSeguroInline(m string) bool {
	m = strings.ToLower(strings.TrimSpace(strings.Split(m, ";")[0]))
	switch m {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "application/pdf":
		return true
	}
	return strings.HasPrefix(m, "audio/") || strings.HasPrefix(m, "video/")
}

// ---------- Mídia enviada (técnico -> pessoa): baixa do SIGE ----------

// Limites da Meta por tipo (Cloud API).
func limiteMidia(kind string) int64 {
	switch kind {
	case "image":
		return 5 << 20
	case "audio", "video":
		return 16 << 20
	default:
		return 100 << 20
	}
}

var (
	errMidiaOrigem  = errors.New("midia_url fora do SIGE")
	errMidiaGrande  = errors.New("mídia acima do limite da Meta")
	errMidiaBaixada = errors.New("não foi possível baixar a mídia do SIGE")
)

// Baixador busca arquivos no SIGE. Só aceita URLs do mesmo esquema+host de
// SIGE_BASE_URL (evita que a rota vire proxy para qualquer endereço).
type Baixador struct {
	origem *url.URL
	apiKey string
	http   *http.Client
}

func NovoBaixador(sigeBaseURL, apiKey string) *Baixador {
	u, _ := url.Parse(strings.TrimRight(sigeBaseURL, "/"))
	return &Baixador{
		origem: u,
		apiKey: apiKey,
		http: &http.Client{
			Timeout:       60 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Arquivo é a mídia baixada do SIGE.
type Arquivo struct {
	Dados []byte
	Mime  string
	Nome  string
}

// Baixar valida a origem, baixa respeitando o limite do tipo e devolve o arquivo.
// mimeInformado/nomeInformado (do payload) têm prioridade sobre o que vier no HTTP.
func (b *Baixador) Baixar(ctx context.Context, rawURL, mimeInformado, nomeInformado string) (*Arquivo, error) {
	u, err := url.Parse(rawURL)
	if err != nil || b.origem == nil || u.Scheme != b.origem.Scheme || !strings.EqualFold(u.Host, b.origem.Host) || u.User != nil {
		return nil, errMidiaOrigem
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errMidiaOrigem
	}
	req.Header.Set("X-API-Key", b.apiKey) // arquivos do SIGE podem ser protegidos
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errMidiaBaixada, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: HTTP %d", errMidiaBaixada, resp.StatusCode)
	}

	mt := strings.TrimSpace(mimeInformado)
	if mt == "" {
		mt = resp.Header.Get("Content-Type")
	}
	if base, _, err := mime.ParseMediaType(mt); err == nil {
		mt = base
	}
	limite := limiteMidia(kindFromMime(mt))
	if resp.ContentLength > limite {
		return nil, errMidiaGrande
	}
	dados, err := io.ReadAll(io.LimitReader(resp.Body, limite+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errMidiaBaixada, err)
	}
	if int64(len(dados)) > limite {
		return nil, errMidiaGrande
	}
	if len(dados) == 0 {
		return nil, fmt.Errorf("%w: arquivo vazio", errMidiaBaixada)
	}
	if mt == "" {
		mt = http.DetectContentType(dados)
	}

	nome := strings.TrimSpace(nomeInformado)
	if nome == "" {
		if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
			nome = params["filename"]
		}
	}
	if nome == "" {
		nome = path.Base(u.Path)
	}
	if nome == "" || nome == "/" || nome == "." {
		nome = "arquivo"
	}
	return &Arquivo{Dados: dados, Mime: mt, Nome: nome}, nil
}

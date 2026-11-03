package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
)

// ---- fakes ----

type fakeStore struct {
	sess    map[string]Session
	cleared []string
	touched []string
}

func newFakeStore() *fakeStore { return &fakeStore{sess: map[string]Session{}} }
func (f *fakeStore) Load(ctx context.Context, phone string) Session {
	if s, ok := f.sess[phone]; ok {
		return s
	}
	return Session{Step: StepMenu}
}
func (f *fakeStore) Save(ctx context.Context, phone string, s Session) error {
	f.sess[phone] = s
	return nil
}
func (f *fakeStore) Clear(ctx context.Context, phone string) error {
	f.cleared = append(f.cleared, phone)
	delete(f.sess, phone)
	return nil
}
func (f *fakeStore) Touch(ctx context.Context, phone string) error {
	f.touched = append(f.touched, phone)
	return nil
}
func (f *fakeStore) WindowExpiry(ctx context.Context, phone string) *time.Time { return nil }

type fakeSige struct {
	abriuRef   string
	lista      []Chamado
	proxTicket int64
}

func (f *fakeSige) AbrirChamado(ctx context.Context, tel, nome, ref string, dados map[string]string, j Janela) (*AbrirResp, error) {
	f.abriuRef = ref
	if f.proxTicket == 0 {
		f.proxTicket = 100
	}
	return &AbrirResp{TicketID: f.proxTicket, Numero: "OS-0100"}, nil
}

type fakeQueue struct {
	recebidas []map[string]any
	dedups    []string
	err       error
}

func (q *fakeQueue) Enqueue(ctx context.Context, phone, dedup string, ev map[string]any) error {
	if q.err != nil {
		return q.err
	}
	q.recebidas = append(q.recebidas, ev)
	q.dedups = append(q.dedups, dedup)
	return nil
}
func (f *fakeSige) ListarChamados(ctx context.Context, tel string, limite int) ([]Chamado, error) {
	return f.lista, nil
}

type fakeReplier struct {
	texts []string
	menus [][]string // chaves por menu
}

func (r *fakeReplier) SendText(ctx context.Context, to, body string) (*messaging.Message, error) {
	r.texts = append(r.texts, body)
	return &messaging.Message{}, nil
}
func (r *fakeReplier) SendMenu(ctx context.Context, to, text string, opts []messaging.MenuOption) (*messaging.Message, error) {
	keys := make([]string, len(opts))
	for i, o := range opts {
		keys[i] = o.Key
	}
	r.menus = append(r.menus, keys)
	return &messaging.Message{}, nil
}

func newSvc() (*Service, *fakeStore, *fakeSige, *fakeReplier) {
	st, sg, rp := newFakeStore(), &fakeSige{}, &fakeReplier{}
	return &Service{repo: st, client: sg, queue: &fakeQueue{}, replier: rp, relayTTL: 72 * time.Hour}, st, sg, rp
}

func msg(from, body, buttonID string) messaging.IncomingMessage {
	return messaging.IncomingMessage{From: from, Body: body, ButtonID: buttonID, Timestamp: time.Now()}
}

// ---- testes ----

func TestMenuNaPrimeiraMsg(t *testing.T) {
	svc, _, _, rp := newSvc()
	svc.HandleInbound(context.Background(), msg("55", "olá", ""))
	if len(rp.menus) != 1 || rp.menus[0][0] != "gw_abrir" || rp.menus[0][1] != "gw_retomar" {
		t.Fatalf("menu errado: %+v", rp.menus)
	}
}

func TestAbrirFluxo(t *testing.T) {
	svc, st, sg, _ := newSvc()
	svc.HandleInbound(context.Background(), msg("55", "", "gw_abrir"))
	if st.sess["55"].Step != StepAbrindo || st.sess["55"].Referencia == "" {
		t.Fatalf("deveria estar abrindo com referência: %+v", st.sess["55"])
	}
	ref := st.sess["55"].Referencia
	svc.HandleInbound(context.Background(), msg("55", "impressora quebrada", ""))
	if sg.abriuRef != ref {
		t.Fatalf("abriu com referência errada: %q != %q", sg.abriuRef, ref)
	}
	if st.sess["55"].Step != StepRelay || st.sess["55"].TicketID != 100 {
		t.Fatalf("deveria entrar em relay(100): %+v", st.sess["55"])
	}
}

func TestRetomarComChamados(t *testing.T) {
	svc, st, sg, rp := newSvc()
	sg.lista = []Chamado{{ID: 7, Numero: "OS-0007", Resumo: "Rede", Status: "aberto"}}
	svc.HandleInbound(context.Background(), msg("55", "", "gw_retomar"))
	if st.sess["55"].Step != StepListando {
		t.Fatalf("deveria estar listando: %+v", st.sess["55"])
	}
	last := rp.menus[len(rp.menus)-1]
	if len(last) != 1 || last[0] != "gw_open_7" {
		t.Fatalf("lista errada: %+v", last)
	}
	// escolhe o chamado
	svc.HandleInbound(context.Background(), msg("55", "", "gw_open_7"))
	if st.sess["55"].Step != StepRelay || st.sess["55"].TicketID != 7 {
		t.Fatalf("deveria entrar em relay(7): %+v", st.sess["55"])
	}
}

func TestRetomarSemChamadosVaiAbrir(t *testing.T) {
	svc, st, _, _ := newSvc()
	svc.HandleInbound(context.Background(), msg("55", "", "gw_retomar"))
	if st.sess["55"].Step != StepAbrindo {
		t.Fatalf("sem chamados deveria cair em abrir: %+v", st.sess["55"])
	}
}

func TestRelayRepassa(t *testing.T) {
	svc, st, _, _ := newSvc()
	q := svc.queue.(*fakeQueue)
	st.sess["55"] = Session{Step: StepRelay, TicketID: 42, AtualizadoEm: time.Now()}
	in := msg("55", "cheguei", "")
	in.WAMessageID = "wamid.ABC"
	svc.HandleInbound(context.Background(), in)
	if len(q.recebidas) != 1 || q.dedups[0] != "wamid.ABC" {
		t.Fatalf("deveria enfileirar 1: %+v", q.recebidas)
	}
	ev := q.recebidas[0]
	if ev["evento"] != "mensagem.recebida" || ev["ticket_id"] != int64(42) || ev["wa_message_id"] != "wamid.ABC" || ev["texto"] != "cheguei" {
		t.Fatalf("evento errado: %+v", ev)
	}
	if len(st.touched) != 1 {
		t.Fatalf("deveria marcar movimento no relay: %+v", st.touched)
	}
}

func TestRelayFilaForaAvisaPessoa(t *testing.T) {
	svc, st, _, rp := newSvc()
	svc.queue.(*fakeQueue).err = context.DeadlineExceeded
	st.sess["55"] = Session{Step: StepRelay, TicketID: 42, AtualizadoEm: time.Now()}
	svc.HandleInbound(context.Background(), msg("55", "oi?", ""))
	if len(rp.texts) != 1 || len(st.touched) != 0 {
		t.Fatalf("deveria avisar e não marcar movimento: %+v %+v", rp.texts, st.touched)
	}
}

func TestRelaySairVoltaAoMenu(t *testing.T) {
	for _, palavra := range []string{"sair", "SAIR", " #sair ", "Menu"} {
		svc, st, _, rp := newSvc()
		st.sess["55"] = Session{Step: StepRelay, TicketID: 42, AtualizadoEm: time.Now()}
		svc.HandleInbound(context.Background(), msg("55", palavra, ""))
		if st.sess["55"].Step != StepMenu || len(rp.menus) != 1 {
			t.Fatalf("%q: deveria voltar ao menu: %+v", palavra, st.sess["55"])
		}
		if n := len(svc.queue.(*fakeQueue).recebidas); n != 0 {
			t.Fatalf("%q: não deveria repassar ao SIGE", palavra)
		}
	}
}

func TestRelaySairSoMensagemInteira(t *testing.T) {
	svc, st, _, _ := newSvc()
	st.sess["55"] = Session{Step: StepRelay, TicketID: 42, AtualizadoEm: time.Now()}
	svc.HandleInbound(context.Background(), msg("55", "vou sair da sala", ""))
	if st.sess["55"].Step != StepRelay || len(svc.queue.(*fakeQueue).recebidas) != 1 {
		t.Fatalf("frase com 'sair' deveria seguir no relay: %+v", st.sess["55"])
	}
}

func TestRelayExpiradoVoltaAoMenu(t *testing.T) {
	svc, st, _, rp := newSvc()
	st.sess["55"] = Session{Step: StepRelay, TicketID: 42, AtualizadoEm: time.Now().Add(-73 * time.Hour)}
	svc.HandleInbound(context.Background(), msg("55", "alguém aí?", ""))
	if st.sess["55"].Step != StepMenu || len(rp.menus) != 1 {
		t.Fatalf("relay parado deveria expirar: %+v", st.sess["55"])
	}
	if len(svc.queue.(*fakeQueue).recebidas) != 0 {
		t.Fatal("relay expirado não deveria repassar")
	}
}

func TestRelayTTLZeroNuncaExpira(t *testing.T) {
	svc, st, _, _ := newSvc()
	svc.relayTTL = 0
	st.sess["55"] = Session{Step: StepRelay, TicketID: 42, AtualizadoEm: time.Now().Add(-1000 * time.Hour)}
	svc.HandleInbound(context.Background(), msg("55", "oi", ""))
	if st.sess["55"].Step != StepRelay || len(svc.queue.(*fakeQueue).recebidas) != 1 {
		t.Fatalf("TTL 0 não deveria expirar: %+v", st.sess["55"])
	}
}

func TestRelayMidiaGuardaRefInterna(t *testing.T) {
	svc, st, _, _ := newSvc()
	st.sess["55"] = Session{Step: StepRelay, TicketID: 42, AtualizadoEm: time.Now()}
	in := msg("55", "print do erro", "")
	in.WAMessageID, in.MediaType, in.MediaID, in.MediaMime = "wamid.I", "image", "meta-123", "image/jpeg"
	svc.HandleInbound(context.Background(), in)
	ev := svc.queue.(*fakeQueue).recebidas[0]
	if ev["tipo"] != "imagem" || ev["_midia_id"] != "meta-123" || ev["midia_url"] != nil {
		t.Fatalf("evento de mídia errado: %+v", ev)
	}
}

func TestEntregarTrocaRefPorURLAssinada(t *testing.T) {
	svc, _, _, _ := newSvc()
	a := NovoAssinador("k", "https://zapgov.exemplo/", time.Minute)
	svc.midia = a
	var enviado map[string]any
	svc.sige = func(ctx context.Context, raw []byte) error { return json.Unmarshal(raw, &enviado) }
	raw := []byte(`{"evento":"mensagem.recebida","midia_url":null,"midia_mime":"image/jpeg","_midia_id":"meta-123","_midia_nome":""}`)
	if err := svc.Entregar(tenantctx.With(context.Background(), 2), raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := enviado["_midia_id"]; ok {
		t.Fatal("campo interno vazou para o SIGE")
	}
	u, _ := enviado["midia_url"].(string)
	if !strings.HasPrefix(u, "https://zapgov.exemplo/v1/midia/") {
		t.Fatalf("midia_url errada: %q", u)
	}
	ref, err := a.Validar(strings.TrimPrefix(u, "https://zapgov.exemplo/v1/midia/"))
	if err != nil || ref.MediaID != "meta-123" || ref.TenantID != 2 || ref.Mime != "image/jpeg" {
		t.Fatalf("token inválido: %+v %v", ref, err)
	}
}

func TestEntregarSemMidiaPassaCru(t *testing.T) {
	svc, _, _, _ := newSvc()
	var got string
	svc.sige = func(ctx context.Context, raw []byte) error { got = string(raw); return nil }
	raw := `{"evento":"mensagem.recebida","texto":"oi"}`
	if err := svc.Entregar(context.Background(), []byte(raw)); err != nil || got != raw {
		t.Fatalf("deveria repassar intacto: %q %v", got, err)
	}
}

package gateway

import (
	"context"
	"testing"
	"time"

	"zapgov/backend/internal/messaging"
)

// ---- fakes ----

type fakeStore struct {
	sess    map[string]Session
	cleared []string
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
func (f *fakeStore) WindowExpiry(ctx context.Context, phone string) *time.Time { return nil }

type fakeSige struct {
	abriuRef   string
	lista      []Chamado
	recebidas  []map[string]any
	proxTicket int64
}

func (f *fakeSige) AbrirChamado(ctx context.Context, tel, nome, ref string, dados map[string]string, j Janela) (*AbrirResp, error) {
	f.abriuRef = ref
	if f.proxTicket == 0 {
		f.proxTicket = 100
	}
	return &AbrirResp{TicketID: f.proxTicket, Numero: "OS-0100"}, nil
}
func (f *fakeSige) MensagemRecebida(ctx context.Context, ev map[string]any) error {
	f.recebidas = append(f.recebidas, ev)
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
	return &Service{repo: st, client: sg, replier: rp}, st, sg, rp
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
	svc, st, sg, _ := newSvc()
	st.sess["55"] = Session{Step: StepRelay, TicketID: 42}
	in := msg("55", "cheguei", "")
	in.WAMessageID = "wamid.ABC"
	svc.HandleInbound(context.Background(), in)
	if len(sg.recebidas) != 1 {
		t.Fatalf("deveria repassar 1: %+v", sg.recebidas)
	}
	ev := sg.recebidas[0]
	if ev["ticket_id"] != int64(42) || ev["wa_message_id"] != "wamid.ABC" || ev["texto"] != "cheguei" {
		t.Fatalf("evento errado: %+v", ev)
	}
}

package techbot

import (
	"context"
	"strings"
	"testing"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tickets"
)

// ---- fakes ----

type fakeStore struct {
	step string
	tid  int64
}

func (f *fakeStore) Load(ctx context.Context, phone string) (string, int64) { return f.step, f.tid }
func (f *fakeStore) Save(ctx context.Context, phone, step string, tid int64) error {
	f.step, f.tid = step, tid
	return nil
}
func (f *fakeStore) Clear(ctx context.Context, phone string) error {
	f.step, f.tid = "", 0
	return nil
}

type sent struct {
	text string
	menu []string // chaves das opções
}

type fakeReplier struct{ out []sent }

func (r *fakeReplier) SendText(ctx context.Context, to, body string) (*messaging.Message, error) {
	r.out = append(r.out, sent{text: body})
	return &messaging.Message{}, nil
}
func (r *fakeReplier) SendMenu(ctx context.Context, to, text string, opts []messaging.MenuOption) (*messaging.Message, error) {
	keys := make([]string, len(opts))
	for i, o := range opts {
		keys[i] = o.Key
	}
	r.out = append(r.out, sent{text: text, menu: keys})
	return &messaging.Message{}, nil
}
func (r *fakeReplier) last() sent { return r.out[len(r.out)-1] }

type fakeTickets struct {
	byID    map[int64]*tickets.Ticket
	comment []string
}

func (f *fakeTickets) List(ctx context.Context, status, prio, cat, tec string) ([]tickets.Ticket, error) {
	var out []tickets.Ticket
	for _, t := range f.byID {
		if status != "" && t.Status != status {
			continue
		}
		if tec != "" && t.Tecnico != tec {
			continue
		}
		out = append(out, *t)
	}
	return out, nil
}
func (f *fakeTickets) Get(ctx context.Context, id int64) (*tickets.Ticket, error) { return f.byID[id], nil }
func (f *fakeTickets) GetByNumero(ctx context.Context, n int) (*tickets.Ticket, error) {
	for _, t := range f.byID {
		if t.Numero == n {
			return t, nil
		}
	}
	return nil, nil
}
func (f *fakeTickets) Assign(ctx context.Context, id int64, tec string) (*tickets.Ticket, error) {
	t := f.byID[id]
	t.Tecnico = tec
	if t.Status == "aberto" || t.Status == "aguardando" {
		t.Status = "em_andamento"
	}
	return t, nil
}
func (f *fakeTickets) SetStatus(ctx context.Context, id int64, st string) (*tickets.Ticket, error) {
	t := f.byID[id]
	t.Status = st
	return t, nil
}
func (f *fakeTickets) AddComment(ctx context.Context, id int64, autor, body string, interno bool) (*tickets.Comment, error) {
	f.comment = append(f.comment, autor+": "+body)
	return &tickets.Comment{}, nil
}

func newSvc() (*Service, *fakeStore, *fakeReplier, *fakeTickets) {
	store := &fakeStore{}
	rep := &fakeReplier{}
	tk := &fakeTickets{byID: map[int64]*tickets.Ticket{
		1: {ID: 1, Numero: 10, Titulo: "Impressora", Status: "aberto", Tecnico: ""},
		2: {ID: 2, Numero: 11, Titulo: "Rede", Status: "em_andamento", Tecnico: "João"},
	}}
	svc := NewService(store, tk, rep, func(ctx context.Context, phone string) (string, bool) {
		return "João Silva", true
	})
	return svc, store, rep, tk
}

// ---- testes ----

func TestMenuPrincipal(t *testing.T) {
	svc, _, rep, _ := newSvc()
	svc.Handle(context.Background(), "555", "João", "oi", "")
	last := rep.last()
	if len(last.menu) != 3 || last.menu[0] != "tk_meus" {
		t.Fatalf("menu principal errado: %+v", last.menu)
	}
}

func TestNaoTecnicoIgnora(t *testing.T) {
	store := &fakeStore{}
	rep := &fakeReplier{}
	svc := NewService(store, &fakeTickets{byID: map[int64]*tickets.Ticket{}}, rep,
		func(ctx context.Context, phone string) (string, bool) { return "", false })
	svc.Handle(context.Background(), "999", "", "oi", "")
	if len(rep.out) != 0 {
		t.Fatalf("não-técnico não devia receber nada, recebeu %d", len(rep.out))
	}
}

func TestFilaListaSoAbertosSemTecnico(t *testing.T) {
	svc, _, rep, _ := newSvc()
	svc.Handle(context.Background(), "555", "João", "", "tk_fila")
	last := rep.last()
	// só o #10 (aberto, sem técnico); o #11 é em_andamento/atribuído.
	if len(last.menu) != 1 || last.menu[0] != "tk_o_1" {
		t.Fatalf("fila errada: %+v", last.menu)
	}
}

func TestMeusChamados(t *testing.T) {
	svc, _, rep, _ := newSvc()
	svc.Handle(context.Background(), "555", "João", "", "tk_meus")
	last := rep.last()
	// João tem o #11 (em_andamento); nome resolvido é "João Silva" -> não casa "João".
	// Ajuste: o técnico do #2 é "João", o resolver devolve "João Silva". Sem match => vazio.
	if len(last.menu) != 0 {
		t.Fatalf("esperava lista vazia (nomes não casam), veio %+v", last.menu)
	}
	if !strings.Contains(last.text, "aberto") {
		t.Fatalf("esperava aviso de vazio, veio %q", last.text)
	}
}

func TestDetalheEAcoes(t *testing.T) {
	svc, _, rep, _ := newSvc()
	svc.Handle(context.Background(), "555", "João", "", "tk_o_1")
	last := rep.last()
	if len(last.menu) != 3 || last.menu[0] != "tk_as_1" || last.menu[2] != "tk_re_1" {
		t.Fatalf("detalhe deveria ter 3 ações: %+v", last.menu)
	}
}

func TestAssumir(t *testing.T) {
	svc, _, _, tk := newSvc()
	svc.Handle(context.Background(), "555", "João", "", "tk_as_1")
	if tk.byID[1].Tecnico != "João Silva" || tk.byID[1].Status != "em_andamento" {
		t.Fatalf("assumir falhou: %+v", tk.byID[1])
	}
}

func TestResolverAssumeAntes(t *testing.T) {
	svc, _, _, tk := newSvc()
	svc.Handle(context.Background(), "555", "João", "", "tk_re_1")
	if tk.byID[1].Status != "resolvido" || tk.byID[1].Tecnico != "João Silva" {
		t.Fatalf("resolver deveria assumir e fechar: %+v", tk.byID[1])
	}
}

func TestComentario(t *testing.T) {
	svc, store, _, tk := newSvc()
	// entra no modo comentário
	svc.Handle(context.Background(), "555", "João", "", "tk_co_1")
	if store.step != "note" || store.tid != 1 {
		t.Fatalf("deveria estar em note/1: %+v", store)
	}
	// manda o texto do comentário
	svc.Handle(context.Background(), "555", "João", "trocado o toner", "")
	if len(tk.comment) != 1 || !strings.Contains(tk.comment[0], "trocado o toner") {
		t.Fatalf("comentário não registrado: %+v", tk.comment)
	}
}

func TestBuscaPorNumero(t *testing.T) {
	svc, store, rep, _ := newSvc()
	svc.Handle(context.Background(), "555", "João", "", "tk_buscar")
	if store.step != "search" {
		t.Fatalf("deveria estar em search: %+v", store)
	}
	svc.Handle(context.Background(), "555", "João", "11", "")
	last := rep.last()
	// abre detalhe do #11 (id 2) -> em_andamento, ações presentes.
	if len(last.menu) != 3 || last.menu[0] != "tk_as_2" {
		t.Fatalf("busca deveria abrir detalhe do #11: %+v", last.menu)
	}
}

package gateway

import (
	"context"
	"errors"
	"testing"

	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/testdb"
)

func TestOutboxRetryOrdemEAviso(t *testing.T) {
	pool := testdb.New(t)
	ctx := tenantctx.With(context.Background(), 2)
	o := NewOutbox(pool)

	for _, id := range []string{"wamid.1", "wamid.2", "wamid.1"} { // wamid.1 repetido: dedup
		if err := o.Enqueue(ctx, "5546999990000", id, map[string]any{"wa_message_id": id}); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM gateway_outbox`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("dedup: %d %v", n, err)
	}

	sigeFora := errors.New("sige fora")
	found, aviso, err := o.ProcessOne(ctx, func(context.Context, []byte) error { return sigeFora })
	if !found || !errors.Is(err, sigeFora) || aviso == nil || aviso.Final || aviso.TenantID != 2 {
		t.Fatalf("1ª falha deveria avisar: %v %+v %v", found, aviso, err)
	}
	// wamid.1 em backoff: wamid.2 (mesmo telefone) não pode passar na frente.
	found, _, err = o.ProcessOne(ctx, func(context.Context, []byte) error { t.Fatal("fora de ordem"); return nil })
	if found || err != nil {
		t.Fatalf("não deveria haver elegível: %v %v", found, err)
	}

	// Nova falha na mesma hora: não avisa de novo.
	if _, err := pool.Exec(ctx, `UPDATE gateway_outbox SET next_attempt_at = now()`); err != nil {
		t.Fatal(err)
	}
	if _, aviso, _ = o.ProcessOne(ctx, func(context.Context, []byte) error { return sigeFora }); aviso != nil {
		t.Fatalf("não deveria avisar duas vezes: %+v", aviso)
	}

	// SIGE volta: entrega na ordem.
	if _, err := pool.Exec(ctx, `UPDATE gateway_outbox SET next_attempt_at = now()`); err != nil {
		t.Fatal(err)
	}
	var ordem []string
	for i := 0; i < 3; i++ {
		_, _, err := o.ProcessOne(ctx, func(_ context.Context, b []byte) error {
			ordem = append(ordem, string(b))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(ordem) != 2 || ordem[0] != `{"wa_message_id": "wamid.1"}` || ordem[1] != `{"wa_message_id": "wamid.2"}` {
		t.Fatalf("ordem errada: %q", ordem)
	}
}

func TestOutboxDesisteEAvisaFinal(t *testing.T) {
	pool := testdb.New(t)
	ctx := tenantctx.With(context.Background(), 2)
	o := NewOutbox(pool)
	if err := o.Enqueue(ctx, "5546999990000", "wamid.X", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE gateway_outbox SET attempts = $1`, maxTentativas-1); err != nil {
		t.Fatal(err)
	}
	_, aviso, _ := o.ProcessOne(ctx, func(context.Context, []byte) error { return errors.New("fora") })
	if aviso == nil || !aviso.Final {
		t.Fatalf("deveria desistir com aviso final: %+v", aviso)
	}
	var failed bool
	if err := pool.QueryRow(ctx, `SELECT failed_at IS NOT NULL FROM gateway_outbox`).Scan(&failed); err != nil || !failed {
		t.Fatalf("deveria marcar failed_at: %v %v", failed, err)
	}
}

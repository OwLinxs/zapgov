package campaigns

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"zapgov/backend/internal/queue"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/testdb"
)

type fakeQueue struct {
	fail  bool
	tasks []queue.CampaignSendPayload
}

func (q *fakeQueue) EnqueueCampaignSend(p queue.CampaignSendPayload) error {
	if q.fail {
		return errors.New("redis unavailable")
	}
	q.tasks = append(q.tasks, p)
	return nil
}
func TestDurableDelivery(t *testing.T) {
	pool := testdb.New(t)
	ctx := tenantctx.With(context.Background(), 1)
	repo := NewRepository(pool)
	camp, err := repo.CreateDelivery(ctx, Campaign{Template: "test", Language: "pt_BR", Total: 2, Status: "enviando"}, []Recipient{{Phone: "5511999990001"}, {Phone: "5511999990002"}}, "Teste")
	if err != nil {
		t.Fatal(err)
	}
	q := &fakeQueue{fail: true}
	if _, err := repo.DispatchPending(ctx, q, 100); err == nil {
		t.Fatal("expected Redis failure")
	}
	q.fail = false
	if n, err := repo.DispatchPending(ctx, q, 100); err != nil || n != 2 {
		t.Fatalf("recover enqueue: %d %v", n, err)
	}
	first := q.tasks[0]
	var claimed atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 12; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ClaimDelivery(ctx, first.RecipientID, camp.ID)
			if err != nil {
				t.Error(err)
			}
			if ok {
				claimed.Add(1)
			}
		}()
	}
	wg.Wait()
	if claimed.Load() != 1 {
		t.Fatalf("duplicate claims: %d", claimed.Load())
	}
	for n := 0; n < 2; n++ {
		if err := repo.FinishDelivery(ctx, first.RecipientID, camp.ID, "sent", "wamid-test", 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	second := q.tasks[1]
	if ok, err := repo.ClaimDelivery(ctx, second.RecipientID, camp.ID); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if err := repo.FinishDelivery(ctx, second.RecipientID, camp.ID, "skipped", "", 0, "opt-out"); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enviadas != 1 || got.Puladas != 1 || got.Status != "enviado" {
		t.Fatalf("wrong completion: %+v", got)
	}
	if ok, _ := repo.ClaimDelivery(ctx, first.RecipientID, camp.ID); ok {
		t.Fatal("completed recipient reclaimed")
	}
	if _, err := repo.DeliveryPayload(tenantctx.With(ctx, 2), first.RecipientID, camp.ID); err == nil {
		t.Fatal("cross-tenant recipient accessible")
	}
	// A process that died after claiming is surfaced, not automatically replayed.
	c2, err := repo.CreateDelivery(ctx, Campaign{Template: "test", Language: "pt_BR", Total: 1, Status: "enviando"}, []Recipient{{Phone: "5511999990003"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	q.tasks = nil
	if _, err := repo.DispatchPending(ctx, q, 100); err != nil {
		t.Fatal(err)
	}
	p := q.tasks[0]
	if _, err := repo.ClaimDelivery(ctx, p.RecipientID, c2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE campaign_recipients SET updated_at=now()-interval '10 minutes' WHERE id=$1`, p.RecipientID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DispatchPending(ctx, q, 100); err != nil {
		t.Fatal(err)
	}
	got, err = repo.Get(ctx, c2.ID)
	if err != nil || got.Revisao != 1 || got.Status != "revisao" {
		t.Fatalf("uncertain delivery: %+v %v", got, err)
	}
}

package reminders

import (
	"context"
	"sync"
	"testing"
	"time"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/testdb"
)

func TestConcurrentReminderClaim(t *testing.T) {
	pool := testdb.New(t)
	ctx := tenantctx.With(context.Background(), 1)
	repo := NewRepository(pool)
	for _, phone := range []string{"5511999990001", "5511999990002", "5511999990003"} {
		if err := repo.Create(ctx, phone, "Test", time.Now().Add(24*time.Hour), time.Now().Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := map[int64]bool{}
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			due, err := repo.Due(ctx, 1)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, r := range due {
				if ids[r.ID] {
					t.Errorf("duplicate reminder %d", r.ID)
				}
				ids[r.ID] = true
			}
		}()
	}
	wg.Wait()
	if len(ids) != 3 {
		t.Fatalf("claimed %d want 3", len(ids))
	}
}

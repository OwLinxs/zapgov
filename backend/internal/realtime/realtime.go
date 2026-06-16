// Package realtime entrega eventos ao vivo para o front via SSE (Server-Sent Events),
// substituindo o polling do inbox. Os eventos são isolados por tenant.
package realtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// Event é o que empurramos para os clientes conectados.
type Event struct {
	TenantID       int64  `json:"-"`    // isolamento: só entregue a clientes do mesmo tenant
	Type           string `json:"type"` // message | status | conversation
	ConversationID int64  `json:"conversation_id,omitempty"`
	Name           string `json:"name,omitempty"`    // nome do contato (msg recebida)
	Preview        string `json:"preview,omitempty"` // prévia do texto (para notificação)
}

type client struct {
	ch     chan Event
	tenant int64
}

// Hub mantém os clientes conectados e faz o fan-out dos eventos por tenant.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

func (h *Hub) subscribe(tenant int64) *client {
	cl := &client{ch: make(chan Event, 16), tenant: tenant}
	h.mu.Lock()
	h.clients[cl] = struct{}{}
	h.mu.Unlock()
	return cl
}

func (h *Hub) unsubscribe(cl *client) {
	h.mu.Lock()
	if _, ok := h.clients[cl]; ok {
		delete(h.clients, cl)
		close(cl.ch)
	}
	h.mu.Unlock()
}

// Publish entrega o evento só aos clientes do mesmo tenant (não bloqueia se o
// buffer encher).
func (h *Hub) Publish(e Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for cl := range h.clients {
		if cl.tenant != e.TenantID {
			continue
		}
		select {
		case cl.ch <- e:
		default: // cliente lento: descarta para não travar
		}
	}
}

// Register monta o endpoint SSE. Autenticação via ?token= (EventSource não manda
// header Authorization). resolve valida o token e devolve o tenant do cliente.
func (h *Hub) Register(r fiber.Router, resolve func(token string) (int64, bool)) {
	r.Get("/events", func(c *fiber.Ctx) error {
		tenant, ok := resolve(c.Query("token"))
		if !ok {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		// Desabilita o buffering do proxy (Nginx/NPM) p/ esta resposta — sem isso
		// os eventos SSE ficam presos no buffer e chegam atrasados/em lote.
		c.Set("X-Accel-Buffering", "no")

		cl := h.subscribe(tenant)
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			defer h.unsubscribe(cl)
			fmt.Fprint(w, ": conectado\n\n")
			if w.Flush() != nil {
				return
			}
			// Heartbeat: proxies derrubam conexão parada (~60s). Um comentário
			// SSE a cada 25s mantém o stream vivo e os eventos chegando na hora.
			tick := time.NewTicker(25 * time.Second)
			defer tick.Stop()
			for {
				select {
				case e, ok := <-cl.ch:
					if !ok {
						return
					}
					b, _ := json.Marshal(e)
					if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
						return
					}
					if w.Flush() != nil {
						return
					}
				case <-tick.C:
					if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
						return
					}
					if w.Flush() != nil {
						return
					}
				}
			}
		}))
		return nil
	})
}

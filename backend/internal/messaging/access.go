package messaging

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"zapgov/backend/internal/tenantctx"
)

// authorize applies the same sector boundary to lists, direct IDs and sends.
func (h *Handler) authorize(c *fiber.Ctx) error {
	papel, _ := c.Locals("papel").(string)
	setor, _ := c.Locals("setor").(string)
	allowed := setoresPermitidos(papel, setor, h.routeBySetor(c), h.visGroups(c))
	c.Locals("allowed_sectors", allowed)
	path := c.Path()
	var convID int64
	var err error
	switch {
	case strings.HasPrefix(path, "/conversations/") && path != "/conversations/resolve-bulk":
		id, e := c.ParamsInt("id")
		if e != nil || id <= 0 {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		convID = int64(id)
	case strings.HasPrefix(path, "/sessions/"):
		id, e := c.ParamsInt("id")
		if e != nil || id <= 0 {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		err = h.svc.repo.pool.QueryRow(c.Context(), `SELECT conversation_id FROM sessions WHERE id=$1 AND tenant_id=$2`, id, tenantctx.From(c.Context())).Scan(&convID)
	case strings.HasPrefix(path, "/media/"):
		err = h.svc.repo.pool.QueryRow(c.Context(), `SELECT conversation_id FROM messages WHERE media_id=$1 AND tenant_id=$2 LIMIT 1`, c.Params("id"), tenantctx.From(c.Context())).Scan(&convID)
	case path == "/conversations/resolve-bulk":
		var in struct {
			IDs []int64 `json:"ids"`
		}
		if c.BodyParser(&in) != nil || len(in.IDs) == 0 || len(in.IDs) > 500 {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		for _, id := range in.IDs {
			if !h.svc.repo.CanAccess(c.Context(), id, allowed) {
				return c.SendStatus(fiber.StatusNotFound)
			}
		}
	case c.Method() == fiber.MethodPost && strings.HasPrefix(path, "/messages/"):
		var to string
		if path == "/messages/upload" {
			to = c.FormValue("to")
		} else {
			var in struct {
				To string `json:"to"`
			}
			if c.BodyParser(&in) != nil {
				return c.SendStatus(fiber.StatusBadRequest)
			}
			to = in.To
		}
		if to == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}
		convID, err = h.svc.ConversationIDByPhone(c.Context(), to)
		// A new conversation may be created in the sender's sector.
		if err == nil && convID == 0 && allowed != nil && len(allowed) == 0 {
			return c.SendStatus(fiber.StatusForbidden)
		}
	}
	if err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if convID > 0 && !h.svc.repo.CanAccess(c.Context(), convID, allowed) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return c.Next()
}

func (r *Repository) CanAccess(ctx context.Context, convID int64, sectors []string) bool {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversations WHERE id=$1 AND tenant_id=$2 AND ($3::boolean OR setor=ANY($4::text[])))`, convID, tenantctx.From(ctx), sectors == nil, sectors).Scan(&ok)
	return err == nil && ok
}

func (r *Repository) ListRecentScoped(ctx context.Context, limit int, sectors []string) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `SELECT `+msgCols+` FROM messages WHERE tenant_id=$1 AND conversation_id IN
 (SELECT id FROM conversations WHERE tenant_id=$1 AND ($2::boolean OR setor=ANY($3::text[]))) ORDER BY created_at DESC LIMIT $4`, tenantctx.From(ctx), sectors == nil, sectors, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		m, e := scanMessage(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

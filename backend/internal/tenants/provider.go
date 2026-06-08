package tenants

import (
	"context"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/messaging/whatsapp"
	"zapgov/backend/internal/tenantctx"
)

// ProviderResolver devolve uma função que escolhe o provider Meta do tenant do
// context. Tenants sem credenciais próprias (ou tenant 1) caem no provider
// padrão (variáveis de ambiente). Os providers são cacheados por tenant e o
// cache é invalidado quando as credenciais mudam (SetCreds).
func (r *Repository) ProviderResolver(def messaging.MessageProvider) func(ctx context.Context) messaging.MessageProvider {
	return func(ctx context.Context) messaging.MessageProvider {
		tid := tenantctx.From(ctx)
		if tid <= 1 {
			return def // tenant da plataforma usa o provider do .env
		}

		r.provMu.RLock()
		p, ok := r.provCache[tid]
		r.provMu.RUnlock()
		if ok {
			return p
		}

		creds, err := r.Creds(ctx, tid)
		if err != nil || creds.PhoneNumberID == "" || creds.Token == "" {
			return def // sem credenciais próprias: usa o padrão
		}
		built := whatsapp.New(creds.PhoneNumberID, creds.Token, creds.AppSecret, creds.APIVersion)

		r.provMu.Lock()
		r.provCache[tid] = built
		r.provMu.Unlock()
		return built
	}
}

// invalidateProvider remove o provider cacheado de um tenant (creds mudaram).
func (r *Repository) invalidateProvider(tid int64) {
	r.provMu.Lock()
	delete(r.provCache, tid)
	r.provMu.Unlock()
}

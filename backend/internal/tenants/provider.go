package tenants

import (
	"context"
	"fmt"
	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/messaging/whatsapp"
	"zapgov/backend/internal/tenantctx"
)

// Resolve on each operation so API and worker observe rotations immediately.
func (r *Repository) ProviderResolver(def messaging.MessageProvider) func(context.Context) messaging.MessageProvider {
	return func(ctx context.Context) messaging.MessageProvider {
		tid := tenantctx.From(ctx)
		creds, err := r.Creds(ctx, tid)
		if err != nil {
			return messaging.UnavailableProvider{Err: fmt.Errorf("%w: %v", messaging.ErrProviderUnavailable, err)}
		}
		if tid == 1 && creds.PhoneNumberID == "" && creds.Token == "" {
			return def
		}
		if creds.PhoneNumberID == "" || creds.Token == "" {
			return messaging.UnavailableProvider{Err: messaging.ErrProviderUnavailable}
		}
		return whatsapp.New(creds.PhoneNumberID, creds.Token, creds.AppSecret, creds.APIVersion)
	}
}

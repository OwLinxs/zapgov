// Package tenantctx carrega o id do tenant (órgão) ao longo de uma operação.
//
// Funciona tanto no caminho HTTP — onde o middleware injeta via Fiber Locals
// (que escreve no UserValue do fasthttp.RequestCtx, lido por ctx.Value) — quanto
// no caminho assíncrono (fila/webhook), onde se usa context.WithValue.
//
// Os repositórios leem From(ctx) dentro de cada query para isolar os dados.
package tenantctx

import "context"

type ctxKey struct{}

// Key é a chave usada para guardar o tenant_id (exportada para o middleware
// HTTP injetar via Fiber Locals com a mesma chave que From() lê).
var Key = ctxKey{}

// With anexa o tenant_id a um context (caminho assíncrono).
func With(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, Key, id)
}

// From devolve o tenant_id do context, ou 0 se ausente.
func From(ctx context.Context) int64 {
	switch v := ctx.Value(Key).(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

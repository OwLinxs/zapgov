package tenants

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

const secretPrefix = "enc:v1:"

func secretCipher() (cipher.AEAD, error) {
	raw := os.Getenv("META_CREDENTIALS_KEY")
	if raw == "" {
		return nil, fmt.Errorf("configure META_CREDENTIALS_KEY para armazenar credenciais Meta")
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("META_CREDENTIALS_KEY deve conter 64 caracteres hexadecimais")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func sealSecret(value, aad string) (string, error) {
	if value == "" {
		return "", nil
	}
	a, err := secretCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := a.Seal(nonce, nonce, []byte(value), []byte(aad))
	return secretPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}
func openSecret(value, aad string) (string, error) {
	if !strings.HasPrefix(value, secretPrefix) {
		return value, nil
	} // legacy values are sealed at startup
	a, err := secretCipher()
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, secretPrefix))
	if err != nil || len(raw) < a.NonceSize() {
		return "", fmt.Errorf("credencial criptografada inválida")
	}
	plain, err := a.Open(nil, raw[:a.NonceSize()], raw[a.NonceSize():], []byte(aad))
	if err != nil {
		return "", fmt.Errorf("não foi possível abrir credencial Meta")
	}
	return string(plain), nil
}
func transformSecrets(id int64, m MetaCreds, fn func(string, string) (string, error)) (MetaCreds, error) {
	fields := []struct {
		name  string
		value *string
	}{{"token", &m.Token}, {"app_secret", &m.AppSecret}, {"verify_token", &m.WebhookVerifyToken}}
	for _, f := range fields {
		v, err := fn(*f.value, fmt.Sprintf("tenant:%d:%s", id, f.name))
		if err != nil {
			return MetaCreds{}, err
		}
		*f.value = v
	}
	return m, nil
}

// SealExisting migrates plaintext credentials transactionally without printing them.
func (r *Repository) SealExisting(ctx context.Context) error {
	if os.Getenv("META_CREDENTIALS_KEY") == "" {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,wa_token,wa_app_secret,webhook_verify_token FROM tenants FOR UPDATE`)
	if err != nil {
		return err
	}
	type row struct {
		id int64
		m  MetaCreds
	}
	entries, err := pgx.CollectRows(rows, func(rr pgx.CollectableRow) (row, error) {
		var x row
		e := rr.Scan(&x.id, &x.m.Token, &x.m.AppSecret, &x.m.WebhookVerifyToken)
		return x, e
	})
	if err != nil {
		return err
	}
	for _, x := range entries {
		plain, err := transformSecrets(x.id, x.m, openSecret)
		if err != nil {
			return err
		}
		if strings.HasPrefix(x.m.Token, secretPrefix) && (x.m.AppSecret == "" || strings.HasPrefix(x.m.AppSecret, secretPrefix)) && (x.m.WebhookVerifyToken == "" || strings.HasPrefix(x.m.WebhookVerifyToken, secretPrefix)) {
			continue
		}
		sealed, err := transformSecrets(x.id, plain, sealSecret)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE tenants SET wa_token=$2,wa_app_secret=$3,webhook_verify_token=$4 WHERE id=$1`, x.id, sealed.Token, sealed.AppSecret, sealed.WebhookVerifyToken); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SealIntegrationSecret protects credentials with tenant/destination binding.
func SealIntegrationSecret(value, binding string) (string, error) {
	return sealSecret(value, "integration:"+binding)
}

func OpenIntegrationSecret(value, binding string) (string, error) {
	if !strings.HasPrefix(value, secretPrefix) {
		return "", fmt.Errorf("credencial inválida")
	}
	return openSecret(value, "integration:"+binding)
}

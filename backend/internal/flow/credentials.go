package flow

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"zapgov/backend/internal/tenants"
)

func credentialBinding(tenantID int64, base string) string {
	return fmt.Sprintf("tenant:%d:sige-ti:%s", tenantID, base)
}

// Work on raw maps to preserve React Flow metadata and unknown node fields.
func transformGraph(raw json.RawMessage, fn func(string, map[string]json.RawMessage) error) (json.RawMessage, error) {
	var graph map[string]json.RawMessage
	if err := json.Unmarshal(raw, &graph); err != nil {
		return nil, err
	}
	var nodes []map[string]json.RawMessage
	if err := json.Unmarshal(graph["nodes"], &nodes); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, node := range nodes {
		var id string
		_ = json.Unmarshal(node["id"], &id)
		if seen[id] {
			return nil, fmt.Errorf("IDs de blocos repetidos")
		}
		seen[id] = true
		var data map[string]json.RawMessage
		if json.Unmarshal(node["data"], &data) != nil || data == nil {
			continue
		}
		if err := fn(id, data); err != nil {
			return nil, err
		}
		node["data"], _ = json.Marshal(data)
	}
	graph["nodes"], _ = json.Marshal(nodes)
	return json.Marshal(graph)
}

func publicGraph(raw json.RawMessage) json.RawMessage {
	out, err := transformGraph(raw, func(_ string, data map[string]json.RawMessage) error {
		var key string
		_ = json.Unmarshal(data["api_key_encrypted"], &key)
		data["api_key_configured"], _ = json.Marshal(key != "")
		delete(data, "api_key")
		delete(data, "api_key_encrypted")
		return nil
	})
	if err != nil {
		return json.RawMessage(`{"nodes":[],"edges":[]}`)
	}
	return out
}

func secureGraph(raw, old json.RawMessage, tenantID int64) (json.RawMessage, error) {
	existing := map[string]apiData{}
	_, err := transformGraph(old, func(id string, data map[string]json.RawMessage) error {
		b, _ := json.Marshal(data)
		var d apiData
		_ = json.Unmarshal(b, &d)
		existing[id] = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	return transformGraph(raw, func(id string, data map[string]json.RawMessage) error {
		b, _ := json.Marshal(data)
		var d apiData
		_ = json.Unmarshal(b, &d)
		delete(data, "api_key")
		delete(data, "api_key_encrypted")
		delete(data, "api_key_configured")
		if d.Integration != "sige_ti" {
			return nil
		}
		d.BaseURL = strings.TrimRight(strings.TrimSpace(d.BaseURL), "/")
		u, err := url.Parse(d.BaseURL)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return fmt.Errorf("Chamar API: URL base inválida")
		}
		key := existing[id].EncryptedKey
		if d.APIKey != "" {
			key, err = tenants.SealIntegrationSecret(d.APIKey, credentialBinding(tenantID, d.BaseURL))
			if err != nil {
				return fmt.Errorf("não foi possível proteger a chave; verifique META_CREDENTIALS_KEY no servidor")
			}
		} else if existing[id].BaseURL != d.BaseURL || key == "" {
			return fmt.Errorf("Chamar API: informe a chave de API para este destino")
		}
		data["base_url"], _ = json.Marshal(d.BaseURL)
		data["api_key_encrypted"], _ = json.Marshal(key)
		return nil
	})
}

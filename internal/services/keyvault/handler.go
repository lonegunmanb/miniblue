package keyvault

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moabukar/miniblue/internal/azerr"
	"github.com/moabukar/miniblue/internal/store"
)

type Secret struct {
	ID          string                 `json:"id"`
	Value       string                 `json:"value,omitempty"`
	ContentType string                 `json:"contentType,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Tags        map[string]string      `json:"tags,omitempty"`
}

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Register(r chi.Router) {
	r.Route("/keyvault/{vaultName}/secrets", func(r chi.Router) {
		r.Get("/", h.ListSecrets)
		r.Route("/{secretName}", func(r chi.Router) {
			r.Put("/", h.SetSecret)
			r.Get("/", h.GetSecret)
			r.Delete("/", h.DeleteSecret)
		})
	})

	// Azure-compatible Key Vault data-plane paths, routed by Host/SNI
	// (for example: https://myvault.vault.azure.net/secrets/my-secret).
	r.Get("/secrets", h.ListSecrets)
	r.Get("/secrets/", h.ListSecrets)
	r.Put("/secrets/{secretName}", h.SetSecret)
	r.Get("/secrets/{secretName}", h.GetSecret)
	r.Get("/secrets/{secretName}/{secretVersion}", h.GetSecret)
	r.Delete("/secrets/{secretName}", h.DeleteSecret)
	r.Get("/deletedsecrets/{secretName}", h.GetDeletedSecret)
	r.Delete("/deletedsecrets/{secretName}", h.PurgeDeletedSecret)
}

func (h *Handler) key(vault, name string) string {
	return "kv:" + vault + ":" + name
}

func (h *Handler) versionKey(vault, name, version string) string {
	return "kvver:" + vault + ":" + name + ":" + version
}

func (h *Handler) deletedKey(vault, name string) string {
	return "kvdel:" + vault + ":" + name
}

func requestVaultName(r *http.Request) string {
	if vaultName := chi.URLParam(r, "vaultName"); vaultName != "" {
		return vaultName
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	return strings.TrimSuffix(host, ".vault.azure.net")
}

func unixNow() int64 {
	return time.Now().UTC().Unix()
}

func deletedSecretResponse(vault, name string, secret interface{}) map[string]interface{} {
	resp := map[string]interface{}{
		"id":         "https://" + vault + ".vault.azure.net/secrets/" + name,
		"recoveryId": "https://" + vault + ".vault.azure.net/deletedsecrets/" + name,
		"attributes": map[string]interface{}{
			"enabled":            true,
			"deletedDate":        unixNow(),
			"scheduledPurgeDate": unixNow() + int64((90 * 24 * time.Hour).Seconds()),
		},
	}
	if s, ok := secret.(Secret); ok {
		resp["value"] = s.Value
		resp["contentType"] = s.ContentType
		resp["tags"] = s.Tags
	}
	return resp
}

func (h *Handler) SetSecret(w http.ResponseWriter, r *http.Request) {
	vault := requestVaultName(r)
	name := chi.URLParam(r, "secretName")

	var body struct {
		Value       string            `json:"value"`
		ContentType string            `json:"contentType"`
		Tags        map[string]string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		azerr.BadRequest(w, "Invalid request body: "+err.Error())
		return
	}

	now := unixNow()
	version := strconv.FormatInt(time.Now().UTC().UnixNano(), 16)
	secret := Secret{
		ID:          "https://" + vault + ".vault.azure.net/secrets/" + name + "/" + version,
		Value:       body.Value,
		ContentType: body.ContentType,
		Tags:        body.Tags,
		Attributes: map[string]interface{}{
			"enabled": true,
			"created": now,
			"updated": now,
		},
	}

	h.store.Set(h.key(vault, name), secret)
	h.store.Set(h.versionKey(vault, name, version), secret)
	h.store.Delete(h.deletedKey(vault, name))
	json.NewEncoder(w).Encode(secret)
}

func (h *Handler) GetSecret(w http.ResponseWriter, r *http.Request) {
	vault := requestVaultName(r)
	name := chi.URLParam(r, "secretName")
	version := chi.URLParam(r, "secretVersion")

	key := h.key(vault, name)
	if version != "" {
		key = h.versionKey(vault, name, version)
	}
	v, ok := h.store.Get(key)
	if !ok {
		azerr.NotFound(w, "Microsoft.KeyVault/vaults/secrets", name)
		return
	}
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	vault := requestVaultName(r)
	name := chi.URLParam(r, "secretName")
	var deleted map[string]interface{}
	if v, ok := h.store.Get(h.key(vault, name)); ok {
		deleted = deletedSecretResponse(vault, name, v)
		h.store.Set(h.deletedKey(vault, name), deleted)
	} else {
		deleted = deletedSecretResponse(vault, name, nil)
	}
	h.store.Delete(h.key(vault, name))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(deleted)
}

func (h *Handler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	vault := requestVaultName(r)
	items := h.store.ListByPrefix("kv:" + vault + ":")
	// Azure Key Vault list returns metadata only, NOT the secret value
	redacted := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if s, ok := item.(Secret); ok {
			redacted = append(redacted, map[string]interface{}{
				"id":         s.ID,
				"attributes": s.Attributes,
			})
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"value": redacted})
}

func (h *Handler) GetDeletedSecret(w http.ResponseWriter, r *http.Request) {
	vault := requestVaultName(r)
	name := chi.URLParam(r, "secretName")
	v, ok := h.store.Get(h.deletedKey(vault, name))
	if !ok {
		azerr.NotFound(w, "Microsoft.KeyVault/vaults/deletedsecrets", name)
		return
	}
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) PurgeDeletedSecret(w http.ResponseWriter, r *http.Request) {
	vault := requestVaultName(r)
	name := chi.URLParam(r, "secretName")
	h.store.Delete(h.deletedKey(vault, name))
	w.WriteHeader(http.StatusNoContent)
}

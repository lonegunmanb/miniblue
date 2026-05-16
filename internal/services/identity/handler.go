package identity

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moabukar/miniblue/internal/azerr"
	"github.com/moabukar/miniblue/internal/store"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	ExpiresOn   string `json:"expires_on"`
	Resource    string `json:"resource"`
}

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Register(r chi.Router) {
	// Managed Identity token endpoint (IMDS)
	r.Get("/metadata/identity/oauth2/token", h.GetToken)
	// Instance Metadata Service
	r.Get("/metadata/instance", h.GetInstanceMetadata)

	r.Route("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.ManagedIdentity/userAssignedIdentities", func(r chi.Router) {
		r.Get("/", h.ListUserAssignedIdentities)
		r.Route("/{identityName}", func(r chi.Router) {
			r.Put("/", h.CreateOrUpdateUserAssignedIdentity)
			r.Patch("/", h.UpdateUserAssignedIdentity)
			r.Get("/", h.GetUserAssignedIdentity)
			r.Delete("/", h.DeleteUserAssignedIdentity)
		})
	})
}

func (h *Handler) GetToken(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		resource = "https://management.azure.com/"
	}

	token := TokenResponse{
		AccessToken: "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6ImxvY2FsLWF6dXJlIn0.miniblue-mock-token",
		TokenType:   "Bearer",
		ExpiresIn:   86400,
		ExpiresOn:   time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		Resource:    resource,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}

func (h *Handler) GetInstanceMetadata(w http.ResponseWriter, r *http.Request) {
	metadata := map[string]interface{}{
		"compute": map[string]interface{}{
			"location":          "eastus",
			"name":              "miniblue-vm",
			"resourceGroupName": "miniblue-rg",
			"subscriptionId":    "00000000-0000-0000-0000-000000000000",
			"vmId":              "miniblue-vm-id",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

func (h *Handler) userAssignedIdentityKey(sub, rg, name string) string {
	return "identity:userAssigned:" + sub + ":" + rg + ":" + name
}

func userAssignedIdentityID(sub, rg, name string) string {
	return "/subscriptions/" + sub + "/resourceGroups/" + rg + "/providers/Microsoft.ManagedIdentity/userAssignedIdentities/" + name
}

func deterministicUUID(seed string) string {
	sum := sha1.Sum([]byte(seed))
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func identityMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func buildUserAssignedIdentityResponse(sub, rg, name string, input, existing map[string]interface{}) map[string]interface{} {
	location, _ := input["location"].(string)
	if location == "" {
		location, _ = existing["location"].(string)
	}
	if location == "" {
		location = "eastus"
	}

	tags := identityMap(input["tags"])
	if tags == nil {
		tags = identityMap(existing["tags"])
	}
	if tags == nil {
		tags = map[string]interface{}{}
	}

	props := identityMap(existing["properties"])
	if props == nil {
		props = map[string]interface{}{}
	}
	for k, v := range identityMap(input["properties"]) {
		props[k] = v
	}
	id := userAssignedIdentityID(sub, rg, name)
	for key, seed := range map[string]string{
		"clientId":    id + ":client",
		"principalId": id + ":principal",
		"tenantId":    "00000000-0000-0000-0000-000000000000",
	} {
		if _, ok := props[key]; !ok {
			if key == "tenantId" {
				props[key] = seed
			} else {
				props[key] = deterministicUUID(seed)
			}
		}
	}

	return map[string]interface{}{
		"id":         id,
		"name":       name,
		"type":       "Microsoft.ManagedIdentity/userAssignedIdentities",
		"location":   location,
		"tags":       tags,
		"properties": props,
	}
}

func (h *Handler) CreateOrUpdateUserAssignedIdentity(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "identityName")

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		azerr.BadRequest(w, "The request content was invalid: "+err.Error())
		return
	}
	if input == nil {
		input = map[string]interface{}{}
	}

	k := h.userAssignedIdentityKey(sub, rg, name)
	existing, exists := h.store.Get(k)
	identity := buildUserAssignedIdentityResponse(sub, rg, name, input, identityMap(existing))
	h.store.Set(k, identity)

	if exists {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	json.NewEncoder(w).Encode(identity)
}

func (h *Handler) UpdateUserAssignedIdentity(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "identityName")

	k := h.userAssignedIdentityKey(sub, rg, name)
	existing, ok := h.store.Get(k)
	if !ok {
		azerr.NotFound(w, "Microsoft.ManagedIdentity/userAssignedIdentities", name)
		return
	}

	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		azerr.BadRequest(w, "The request content was invalid: "+err.Error())
		return
	}
	existingMap := identityMap(existing)
	if tags, ok := patch["tags"]; ok {
		existingMap["tags"] = identityMap(tags)
	}
	identity := buildUserAssignedIdentityResponse(sub, rg, name, existingMap, existingMap)
	h.store.Set(k, identity)
	json.NewEncoder(w).Encode(identity)
}

func (h *Handler) GetUserAssignedIdentity(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "identityName")

	v, ok := h.store.Get(h.userAssignedIdentityKey(sub, rg, name))
	if !ok {
		azerr.NotFound(w, "Microsoft.ManagedIdentity/userAssignedIdentities", name)
		return
	}
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) DeleteUserAssignedIdentity(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "identityName")

	if !h.store.Delete(h.userAssignedIdentityKey(sub, rg, name)) {
		azerr.NotFound(w, "Microsoft.ManagedIdentity/userAssignedIdentities", name)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ListUserAssignedIdentities(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	items := h.store.ListByPrefix("identity:userAssigned:" + sub + ":" + rg + ":")
	json.NewEncoder(w).Encode(map[string]interface{}{"value": items})
}

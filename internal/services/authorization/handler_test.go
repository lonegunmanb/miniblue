package authorization

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moabukar/miniblue/internal/store"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	r := chi.NewRouter()
	NewHandler(store.New()).Register(r)
	return r
}

func do(t *testing.T, h http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decodeMap(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var got map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rr.Body.String())
	}
	return got
}

func TestRoleDefinitionsBuiltinsGetAndFilteredList(t *testing.T) {
	h := newTestServer(t)
	scope := "/subscriptions/sub1"

	get := do(t, h, http.MethodGet, scope+"/providers/Microsoft.Authorization/roleDefinitions/acdd72a7-3385-48ef-bd42-f606fba81ae7", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("GET role definition: want 200, got %d: %s", get.Code, get.Body.String())
	}
	def := decodeMap(t, get)
	props := def["properties"].(map[string]interface{})
	if props["roleName"] != "Reader" || props["type"] != "BuiltInRole" {
		t.Fatalf("unexpected Reader definition: %#v", def)
	}

	list := do(t, h, http.MethodGet, scope+"/providers/Microsoft.Authorization/roleDefinitions?$filter=roleName%20eq%20'Storage%20Blob%20Data%20Contributor'", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("LIST role definitions: want 200, got %d", list.Code)
	}
	body := decodeMap(t, list)
	values := body["value"].([]interface{})
	if len(values) != 1 {
		t.Fatalf("expected one filtered role definition, got %#v", body)
	}
	filteredProps := values[0].(map[string]interface{})["properties"].(map[string]interface{})
	if filteredProps["roleName"] != "Storage Blob Data Contributor" {
		t.Fatalf("unexpected filtered role: %#v", values[0])
	}
}

func TestRoleAssignmentLifecycleAtResourceScope(t *testing.T) {
	h := newTestServer(t)
	scope := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Storage/storageAccounts/acct1"
	path := scope + "/providers/Microsoft.Authorization/roleAssignments/11111111-1111-1111-1111-111111111111"
	roleDefID := "/subscriptions/sub1/providers/Microsoft.Authorization/roleDefinitions/acdd72a7-3385-48ef-bd42-f606fba81ae7"

	create := do(t, h, http.MethodPut, path, map[string]interface{}{
		"properties": map[string]interface{}{
			"principalId":      "principal-1",
			"principalType":    "ServicePrincipal",
			"roleDefinitionId": roleDefID,
		},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("PUT role assignment: want 201, got %d: %s", create.Code, create.Body.String())
	}
	created := decodeMap(t, create)
	if created["id"] != path || created["type"] != "Microsoft.Authorization/roleAssignments" {
		t.Fatalf("unexpected assignment response: %#v", created)
	}
	props := created["properties"].(map[string]interface{})
	if props["principalId"] != "principal-1" || props["roleDefinitionId"] != roleDefID || props["scope"] != scope {
		t.Fatalf("assignment properties not preserved: %#v", props)
	}

	list := do(t, h, http.MethodGet, scope+"/providers/Microsoft.Authorization/roleAssignments", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("LIST assignments: want 200, got %d", list.Code)
	}
	if values := decodeMap(t, list)["value"].([]interface{}); len(values) != 1 {
		t.Fatalf("expected one assignment in list, got %#v", values)
	}

	del := do(t, h, http.MethodDelete, path, nil)
	if del.Code != http.StatusOK {
		t.Fatalf("DELETE assignment: want 200, got %d", del.Code)
	}
	missing := do(t, h, http.MethodGet, path, nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET deleted assignment: want 404, got %d", missing.Code)
	}
}

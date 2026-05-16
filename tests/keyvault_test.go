package tests

import (
	"strings"
	"testing"
)

func TestKeyVaultCRUD(t *testing.T) {
	ts := setupServer()
	defer ts.Close()
	base := ts.URL + "/keyvault/myvault/secrets"

	// Set secret
	resp := doRequest(t, "PUT", base+"/db-pass", `{"value":"supersecret"}`)
	defer resp.Body.Close()
	expectStatus(t, resp, 200)

	m := decodeJSON(t, resp)
	if m["value"] != "supersecret" {
		t.Fatalf("expected value=supersecret, got %v", m["value"])
	}

	// Get
	resp = doRequest(t, "GET", base+"/db-pass", "")
	m = decodeJSON(t, resp)
	if m["value"] != "supersecret" {
		t.Fatalf("get: expected value=supersecret, got %v", m["value"])
	}

	// List
	resp = doRequest(t, "GET", base, "")
	list := decodeJSON(t, resp)
	if list["value"] == nil {
		t.Fatal("expected value array in list response")
	}

	// Delete
	resp = doRequest(t, "DELETE", base+"/db-pass", "")
	resp.Body.Close()

	// Should 404 now
	resp = doRequest(t, "GET", base+"/db-pass", "")
	defer resp.Body.Close()
	expectStatus(t, resp, 404)
}

func TestKeyVaultDataPlaneVaultAzureNetHostCRUD(t *testing.T) {
	ts := setupServer()
	defer ts.Close()
	host := "myvault.vault.azure.net"
	base := ts.URL + "/secrets/db-pass"

	resp := doAppConfigHostRequest(t, "PUT", base+"?api-version=7.4", host, `{"value":"supersecret","contentType":"text/plain","tags":{"env":"test"}}`)
	defer resp.Body.Close()
	expectStatus(t, resp, 200)

	m := decodeJSON(t, resp)
	if m["value"] != "supersecret" || m["contentType"] != "text/plain" {
		t.Fatalf("expected secret value/contentType round trip, got %v", m)
	}
	id, _ := m["id"].(string)
	if !strings.HasPrefix(id, "https://myvault.vault.azure.net/secrets/db-pass/") {
		t.Fatalf("expected versioned vault.azure.net id, got %q", id)
	}
	version := id[strings.LastIndex(id, "/")+1:]

	resp2 := doAppConfigHostRequest(t, "GET", base+"/"+version+"?api-version=7.4", host, "")
	defer resp2.Body.Close()
	expectStatus(t, resp2, 200)
	m = decodeJSON(t, resp2)
	if m["value"] != "supersecret" {
		t.Fatalf("expected versioned get value=supersecret, got %v", m["value"])
	}

	resp3 := doAppConfigHostRequest(t, "GET", ts.URL+"/secrets?api-version=7.4", host, "")
	defer resp3.Body.Close()
	expectStatus(t, resp3, 200)
	list := decodeJSON(t, resp3)
	items := list["value"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected one listed secret, got %d", len(items))
	}
	item := items[0].(map[string]interface{})
	if _, ok := item["value"]; ok {
		t.Fatalf("list response should redact secret value, got %v", item)
	}

	resp4 := doAppConfigHostRequest(t, "DELETE", base+"?api-version=7.4", host, "")
	defer resp4.Body.Close()
	expectStatus(t, resp4, 200)

	resp5 := doAppConfigHostRequest(t, "GET", base+"?api-version=7.4", host, "")
	defer resp5.Body.Close()
	expectStatus(t, resp5, 404)

	resp6 := doAppConfigHostRequest(t, "GET", ts.URL+"/deletedsecrets/db-pass?api-version=7.4", host, "")
	defer resp6.Body.Close()
	expectStatus(t, resp6, 200)
}

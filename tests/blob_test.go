package tests

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestBlobStorageCRUD(t *testing.T) {
	ts := setupServer()
	defer ts.Close()
	base := ts.URL + "/blob/myaccount"

	// Create container
	resp := doRequest(t, "PUT", base+"/mycontainer", "")
	resp.Body.Close()
	expectStatus(t, resp, 201)

	// Upload blob
	resp = doRequest(t, "PUT", base+"/mycontainer/hello.txt", "Hello World!")
	resp.Body.Close()
	expectStatus(t, resp, 201)

	// Download
	resp = doRequest(t, "GET", base+"/mycontainer/hello.txt", "")
	defer resp.Body.Close()
	expectStatus(t, resp, 200)

	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	if buf.String() != "Hello World!" {
		t.Fatalf("expected 'Hello World!', got '%s'", buf.String())
	}

	// Verify content-length header
	if resp.Header.Get("Content-Length") != "12" {
		t.Fatalf("expected Content-Length=12, got %s", resp.Header.Get("Content-Length"))
	}
}

func TestBlobListContentLength(t *testing.T) {
	ts := setupServer()
	defer ts.Close()
	base := ts.URL + "/blob/myaccount/mycontainer"

	doRequest(t, "PUT", base, "").Body.Close()
	doRequest(t, "PUT", base+"/test.txt", "abcdef").Body.Close()

	resp := doRequest(t, "GET", base, "")
	defer resp.Body.Close()
	expectStatus(t, resp, 200)

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Fatalf("expected XML Content-Type, got %s", ct)
	}

	var result struct {
		XMLName         xml.Name `xml:"EnumerationResults"`
		ServiceEndpoint string   `xml:"ServiceEndpoint,attr"`
		ContainerName   string   `xml:"ContainerName,attr"`
		Blobs           struct {
			Items []struct {
				Name       string `xml:"Name"`
				Properties struct {
					ContentLength int64  `xml:"Content-Length"`
					ContentType   string `xml:"Content-Type"`
					BlobType      string `xml:"BlobType"`
					LeaseStatus   string `xml:"LeaseStatus"`
					LeaseState    string `xml:"LeaseState"`
					Etag          string `xml:"Etag"`
					LastModified  string `xml:"Last-Modified"`
				} `xml:"Properties"`
			} `xml:"Blob"`
		} `xml:"Blobs"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode XML response: %v", err)
	}

	if result.ContainerName != "mycontainer" {
		t.Fatalf("expected ContainerName=mycontainer, got %q", result.ContainerName)
	}
	if result.ServiceEndpoint == "" {
		t.Fatalf("expected ServiceEndpoint attribute to be set")
	}
	if len(result.Blobs.Items) != 1 {
		t.Fatalf("expected 1 blob, got %d", len(result.Blobs.Items))
	}
	b := result.Blobs.Items[0]
	if b.Name != "test.txt" {
		t.Fatalf("expected blob name test.txt, got %q", b.Name)
	}
	if b.Properties.ContentLength != 6 {
		t.Fatalf("expected Content-Length=6, got %d", b.Properties.ContentLength)
	}
	if b.Properties.BlobType != "BlockBlob" {
		t.Fatalf("expected BlobType=BlockBlob, got %q", b.Properties.BlobType)
	}
	if b.Properties.LeaseStatus != "unlocked" || b.Properties.LeaseState != "available" {
		t.Fatalf("expected unlocked/available lease, got %q/%q", b.Properties.LeaseStatus, b.Properties.LeaseState)
	}
}

func TestBlobNotFound(t *testing.T) {
	ts := setupServer()
	defer ts.Close()

	resp := doRequest(t, "GET", ts.URL+"/blob/acct/container/nope.txt", "")
	defer resp.Body.Close()
	expectStatus(t, resp, 404)
}

func TestBlobDelete(t *testing.T) {
	ts := setupServer()
	defer ts.Close()
	base := ts.URL + "/blob/myaccount/mycontainer"

	doRequest(t, "PUT", base, "").Body.Close()
	doRequest(t, "PUT", base+"/file.txt", "data").Body.Close()

	resp := doRequest(t, "DELETE", base+"/file.txt", "")
	resp.Body.Close()
	expectStatus(t, resp, 202)

	resp = doRequest(t, "GET", base+"/file.txt", "")
	defer resp.Body.Close()
	expectStatus(t, resp, 404)
}

// TestBlobNotFoundTerminatesResponse verifies that HEAD/GET against a
// non-existent blob returns a *fully terminated* 404 response — i.e. the
// client is not left hanging waiting for an unfinished chunked body. This
// regression broke Terraform's azurerm remote-state backend, whose Lock()
// path issues GetProperties (HEAD) on the state blob and treats a clean 404
// as "blob doesn't exist yet, create it".
func TestBlobNotFoundTerminatesResponse(t *testing.T) {
ts := setupServer()
defer ts.Close()
base := ts.URL + "/blob/myaccount/mycontainer"

doRequest(t, "PUT", base, "").Body.Close()

for _, method := range []string{"HEAD", "GET"} {
resp := doRequest(t, method, base+"/missing.txt", "")
expectStatus(t, resp, 404)

if got := resp.Header.Get("x-ms-error-code"); got != "BlobNotFound" {
t.Errorf("%s: expected x-ms-error-code=BlobNotFound, got %q", method, got)
}
if resp.Header.Get("x-ms-request-id") == "" {
t.Errorf("%s: missing x-ms-request-id header", method)
}
// The response MUST be terminated. Either via Content-Length, or — if
// chunked — via the zero-length terminator (which makes io.ReadAll
// return without error). Without termination, this read would block
// until the test timeout.
if _, err := io.ReadAll(resp.Body); err != nil {
t.Errorf("%s: reading body: %v", method, err)
}
resp.Body.Close()

// Explicit Content-Length avoids Go's chunked fallback (which
// silently truncates HEAD responses without a terminator).
if cl := resp.Header.Get("Content-Length"); cl == "" {
t.Errorf("%s: missing Content-Length on 404 response", method)
}
}
}

package tests

import (
	"bytes"
	"encoding/xml"
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

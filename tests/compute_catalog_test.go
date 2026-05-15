package tests

import "testing"

func TestComputeStaticCatalogEndpoints(t *testing.T) {
	ts := setupServer()
	defer ts.Close()

	base := ts.URL + "/subscriptions/sub1/providers/Microsoft.Compute"
	av := "?api-version=2023-09-01"

	resp := doRequest(t, "GET", base+"/locations/eastus/vmSizes"+av, "")
	expectStatus(t, resp, 200)
	vmSizes := decodeJSON(t, resp)
	resp.Body.Close()
	if len(vmSizes["value"].([]interface{})) == 0 {
		t.Fatalf("expected vmSizes to include static values")
	}
	if vmSizes["value"].([]interface{})[0].(map[string]interface{})["name"] != "Standard_DS1_v2" {
		t.Fatalf("expected Standard_DS1_v2 in vmSizes, got %v", vmSizes["value"])
	}

	resp = doRequest(t, "GET", base+"/skus"+av, "")
	expectStatus(t, resp, 200)
	skus := decodeJSON(t, resp)
	resp.Body.Close()
	if len(skus["value"].([]interface{})) == 0 {
		t.Fatalf("expected skus to include static values")
	}

	resp = doRequest(t, "GET", base+"/locations/eastus/publishers"+av, "")
	expectStatus(t, resp, 200)
	publishers := decodeJSON(t, resp)
	resp.Body.Close()
	if len(publishers["value"].([]interface{})) == 0 {
		t.Fatalf("expected publishers to include static values")
	}

	resp = doRequest(t, "GET", base+"/locations/eastus/publishers/Canonical/artifacttypes/vmimage/offers"+av, "")
	expectStatus(t, resp, 200)
	offers := decodeJSON(t, resp)
	resp.Body.Close()
	offerItems := offers["value"].([]interface{})
	if len(offerItems) == 0 || offerItems[0].(map[string]interface{})["name"] != "0001-com-ubuntu-server-jammy" {
		t.Fatalf("expected Ubuntu offer, got %v", offers["value"])
	}

	resp = doRequest(t, "GET", base+"/locations/eastus/publishers/Canonical/artifacttypes/vmimage/offers/0001-com-ubuntu-server-jammy/skus"+av, "")
	expectStatus(t, resp, 200)
	imageSkus := decodeJSON(t, resp)
	resp.Body.Close()
	imageSkuItems := imageSkus["value"].([]interface{})
	if len(imageSkuItems) == 0 || imageSkuItems[0].(map[string]interface{})["name"] != "22_04-lts" {
		t.Fatalf("expected Ubuntu image sku, got %v", imageSkus["value"])
	}

	resp = doRequest(t, "GET", base+"/locations/eastus/publishers/Canonical/artifacttypes/vmimage/offers/0001-com-ubuntu-server-jammy/skus/22_04-lts/versions"+av, "")
	expectStatus(t, resp, 200)
	versions := decodeJSON(t, resp)
	resp.Body.Close()
	if len(versions["value"].([]interface{})) == 0 {
		t.Fatalf("expected versions to include static values")
	}

	resp = doRequest(t, "GET", base+"/locations/eastus/publishers/Canonical/artifacttypes/vmimage/offers/0001-com-ubuntu-server-jammy/skus/22_04-lts/versions/latest"+av, "")
	expectStatus(t, resp, 200)
	version := decodeJSON(t, resp)
	resp.Body.Close()
	if version["name"] != "latest" {
		t.Fatalf("expected latest image version, got %v", version["name"])
	}
}

package compute

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moabukar/miniblue/internal/azerr"
	"github.com/moabukar/miniblue/internal/store"
)

type Handler struct{}

func NewHandler(_ *store.Store) *Handler {
	return &Handler{}
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/subscriptions/{subscriptionId}/providers/Microsoft.Compute/locations/{location}/vmSizes", h.ListVMSizes)
	r.Get("/subscriptions/{subscriptionId}/providers/Microsoft.Compute/skus", h.ListSKUs)
	r.Route("/subscriptions/{subscriptionId}/providers/Microsoft.Compute/locations/{location}/publishers", func(r chi.Router) {
		r.Get("/", h.ListPublishers)
		r.Get("/{publisherName}/artifacttypes/vmimage/offers", h.ListOffers)
		r.Get("/{publisherName}/artifacttypes/vmimage/offers/{offerName}/skus", h.ListImageSKUs)
		r.Get("/{publisherName}/artifacttypes/vmimage/offers/{offerName}/skus/{skuName}/versions", h.ListImageVersions)
		r.Get("/{publisherName}/artifacttypes/vmimage/offers/{offerName}/skus/{skuName}/versions/{versionName}", h.GetImageVersion)
	})
}

func (h *Handler) ListVMSizes(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"value": []interface{}{
		map[string]interface{}{
			"name":                 "Standard_DS1_v2",
			"numberOfCores":        1,
			"osDiskSizeInMB":       1047552,
			"resourceDiskSizeInMB": 7168,
			"memoryInMB":           3584,
			"maxDataDiskCount":     4,
		},
		map[string]interface{}{
			"name":                 "Standard_DS2_v2",
			"numberOfCores":        2,
			"osDiskSizeInMB":       1047552,
			"resourceDiskSizeInMB": 14336,
			"memoryInMB":           7168,
			"maxDataDiskCount":     8,
		},
	}})
}

func (h *Handler) ListSKUs(w http.ResponseWriter, r *http.Request) {
	loc := "eastus"
	json.NewEncoder(w).Encode(map[string]interface{}{"value": []interface{}{
		computeSKU("Standard_DS1_v2", "virtualMachines", loc, []interface{}{
			capability("vCPUs", "1"),
			capability("MemoryGB", "3.5"),
			capability("MaxDataDiskCount", "4"),
		}),
		computeSKU("Standard_DS2_v2", "virtualMachines", loc, []interface{}{
			capability("vCPUs", "2"),
			capability("MemoryGB", "7"),
			capability("MaxDataDiskCount", "8"),
		}),
		computeSKU("Standard_LRS", "disks", loc, nil),
		computeSKU("Premium_LRS", "disks", loc, nil),
	}})
}

func (h *Handler) ListPublishers(w http.ResponseWriter, r *http.Request) {
	items := []interface{}{}
	for _, img := range imageCatalog() {
		items = append(items, catalogItem(r, img.publisher))
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"value": items})
}

func (h *Handler) ListOffers(w http.ResponseWriter, r *http.Request) {
	publisher := chi.URLParam(r, "publisherName")
	items := []interface{}{}
	for _, img := range imageCatalog() {
		if img.publisher == publisher {
			items = append(items, catalogItem(r, img.offer))
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"value": items})
}

func (h *Handler) ListImageSKUs(w http.ResponseWriter, r *http.Request) {
	publisher := chi.URLParam(r, "publisherName")
	offer := chi.URLParam(r, "offerName")
	items := []interface{}{}
	for _, img := range imageCatalog() {
		if img.publisher == publisher && img.offer == offer {
			items = append(items, catalogItem(r, img.sku))
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"value": items})
}

func (h *Handler) ListImageVersions(w http.ResponseWriter, r *http.Request) {
	publisher := chi.URLParam(r, "publisherName")
	offer := chi.URLParam(r, "offerName")
	sku := chi.URLParam(r, "skuName")
	items := []interface{}{}
	for _, img := range imageCatalog() {
		if img.publisher == publisher && img.offer == offer && img.sku == sku {
			items = append(items, imageVersion(r, img, img.version))
			items = append(items, imageVersion(r, img, "latest"))
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"value": items})
}

func (h *Handler) GetImageVersion(w http.ResponseWriter, r *http.Request) {
	publisher := chi.URLParam(r, "publisherName")
	offer := chi.URLParam(r, "offerName")
	sku := chi.URLParam(r, "skuName")
	version := chi.URLParam(r, "versionName")
	for _, img := range imageCatalog() {
		if img.publisher == publisher && img.offer == offer && img.sku == sku {
			json.NewEncoder(w).Encode(imageVersion(r, img, version))
			return
		}
	}
	azerr.NotFound(w, "Microsoft.Compute/locations/publishers/artifacttypes/vmimage", publisher+"/"+offer+"/"+sku+"/"+version)
}

func computeSKU(name, resourceType, location string, capabilities []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"resourceType": resourceType,
		"name":         name,
		"tier":         "Standard",
		"size":         name,
		"family":       "standardDSv2Family",
		"locations":    []interface{}{location},
		"locationInfo": []interface{}{map[string]interface{}{"location": location, "zones": []interface{}{}}},
		"capabilities": capabilities,
	}
}

func capability(name, value string) map[string]interface{} {
	return map[string]interface{}{"name": name, "value": value}
}

type imageRef struct {
	publisher string
	offer     string
	sku       string
	version   string
	osType    string
}

func imageCatalog() []imageRef {
	return []imageRef{
		{publisher: "Canonical", offer: "0001-com-ubuntu-server-jammy", sku: "22_04-lts", version: "22.04.202405210", osType: "Linux"},
		{publisher: "OpenLogic", offer: "CentOS", sku: "7_9", version: "7.9.2024052100", osType: "Linux"},
		{publisher: "MicrosoftWindowsServer", offer: "WindowsServer", sku: "2019-Datacenter", version: "17763.5936.240505", osType: "Windows"},
	}
}

func catalogItem(r *http.Request, name string) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"location": chi.URLParam(r, "location"),
		"id":       r.URL.Path + "/" + name,
	}
}

func imageVersion(r *http.Request, img imageRef, version string) map[string]interface{} {
	if version == "" {
		version = img.version
	}
	return map[string]interface{}{
		"name":     version,
		"location": chi.URLParam(r, "location"),
		"id":       r.URL.Path,
		"properties": map[string]interface{}{
			"storageProfile": map[string]interface{}{
				"osDiskImage":    map[string]interface{}{"operatingSystem": img.osType},
				"dataDiskImages": []interface{}{},
			},
		},
	}
}

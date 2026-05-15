package virtualmachines

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moabukar/miniblue/internal/azerr"
	"github.com/moabukar/miniblue/internal/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Register(r chi.Router) {
	r.Route("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachines", func(r chi.Router) {
		r.Get("/", h.List)
		r.Route("/{vmName}", func(r chi.Router) {
			r.Put("/", h.CreateOrUpdate)
			r.Patch("/", h.Update)
			r.Get("/", h.Get)
			r.Delete("/", h.Delete)
			r.Get("/instanceView", h.InstanceView)
			r.Post("/start", h.Start)
			r.Post("/powerOff", h.PowerOff)
			r.Post("/restart", h.Restart)
			r.Post("/deallocate", h.Deallocate)
			r.Post("/redeploy", h.Redeploy)
			r.Route("/extensions", func(r chi.Router) {
				r.Get("/", h.ListExtensions)
				r.Route("/{extensionName}", func(r chi.Router) {
					r.Put("/", h.CreateOrUpdateExtension)
					r.Patch("/", h.UpdateExtension)
					r.Get("/", h.GetExtension)
					r.Delete("/", h.DeleteExtension)
				})
			})
		})
	})
	r.Get("/subscriptions/{subscriptionId}/providers/Microsoft.Compute/virtualMachines", h.ListInSubscription)
}

func (h *Handler) key(sub, rg, name string) string {
	return "vm:" + sub + ":" + rg + ":" + name
}

func (h *Handler) extensionKey(sub, rg, vm, name string) string {
	return "vmext:" + sub + ":" + rg + ":" + vm + ":" + name
}

func resourceID(sub, rg, name string) string {
	return "/subscriptions/" + sub + "/resourceGroups/" + rg + "/providers/Microsoft.Compute/virtualMachines/" + name
}

func extensionResourceID(sub, rg, vm, name string) string {
	return resourceID(sub, rg, vm) + "/extensions/" + name
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func asArray(v interface{}) []interface{} {
	if a, ok := v.([]interface{}); ok {
		return a
	}
	return []interface{}{}
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	dst := map[string]interface{}{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func recursiveCopy(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		m := map[string]interface{}{}
		for k, item := range x {
			m[k] = recursiveCopy(item)
		}
		return m
	case []interface{}:
		items := make([]interface{}, 0, len(x))
		for _, item := range x {
			items = append(items, recursiveCopy(item))
		}
		return items
	default:
		return v
	}
}

func mergePatch(existing, patch map[string]interface{}) map[string]interface{} {
	merged := copyMap(existing)
	for k, v := range patch {
		if k == "properties" {
			props := copyMap(asMap(existing["properties"]))
			for pk, pv := range asMap(v) {
				props[pk] = pv
			}
			merged[k] = props
			continue
		}
		merged[k] = v
	}
	return merged
}

func firstString(values ...interface{}) string {
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func powerState(vm map[string]interface{}) string {
	state, _ := vm["_powerState"].(string)
	if state == "" {
		return "PowerState/running"
	}
	return state
}

func instanceViewFor(state string) map[string]interface{} {
	now := time.Now().UTC().Format(time.RFC3339)
	display := strings.TrimPrefix(state, "PowerState/")
	return map[string]interface{}{
		"vmAgent": map[string]interface{}{
			"vmAgentVersion": "miniblue",
			"statuses": []interface{}{
				map[string]interface{}{
					"code":          "ProvisioningState/succeeded",
					"displayStatus": "Ready",
					"level":         "Info",
					"time":          now,
				},
			},
		},
		"statuses": []interface{}{
			map[string]interface{}{
				"code":          "ProvisioningState/succeeded",
				"displayStatus": "Provisioning succeeded",
				"level":         "Info",
				"time":          now,
			},
			map[string]interface{}{
				"code":          state,
				"displayStatus": "VM " + display,
				"level":         "Info",
				"time":          now,
			},
		},
	}
}

func buildResponse(sub, rg, name string, input map[string]interface{}, existing map[string]interface{}) map[string]interface{} {
	id := resourceID(sub, rg, name)
	props := asMap(input["properties"])
	if props == nil {
		props = map[string]interface{}{}
	}
	existingProps := asMap(existing["properties"])

	location := firstString(input["location"], existing["location"])
	if location == "" {
		location = "eastus"
	}
	tags := asMap(input["tags"])
	if tags == nil {
		tags = asMap(existing["tags"])
	}
	if tags == nil {
		tags = map[string]interface{}{}
	}

	vmProps := copyMap(props)
	if len(vmProps) == 0 {
		vmProps = copyMap(existingProps)
	}
	if _, ok := vmProps["hardwareProfile"]; !ok {
		if v, ok := existingProps["hardwareProfile"]; ok {
			vmProps["hardwareProfile"] = v
		}
	}
	if _, ok := vmProps["storageProfile"]; !ok {
		if v, ok := existingProps["storageProfile"]; ok {
			vmProps["storageProfile"] = v
		}
	}
	if _, ok := vmProps["osProfile"]; !ok {
		if v, ok := existingProps["osProfile"]; ok {
			vmProps["osProfile"] = v
		}
	}
	if _, ok := vmProps["networkProfile"]; !ok {
		if v, ok := existingProps["networkProfile"]; ok {
			vmProps["networkProfile"] = v
		}
	}
	vmProps["provisioningState"] = "Succeeded"

	state := firstString(existing["_powerState"])
	if state == "" {
		state = "PowerState/running"
	}

	resp := map[string]interface{}{
		"id":          id,
		"name":        name,
		"type":        "Microsoft.Compute/virtualMachines",
		"location":    location,
		"tags":        tags,
		"properties":  vmProps,
		"_powerState": state,
	}
	for _, field := range []string{"identity", "plan", "sku", "zones", "extendedLocation"} {
		if v, ok := input[field]; ok {
			resp[field] = v
		} else if v, ok := existing[field]; ok {
			resp[field] = v
		}
	}
	return resp
}

func sanitizeVM(vm map[string]interface{}, includeInstanceView bool) map[string]interface{} {
	out := recursiveCopy(vm).(map[string]interface{})
	delete(out, "_powerState")
	props := asMap(out["properties"])
	if props == nil {
		return out
	}
	if includeInstanceView {
		props["instanceView"] = instanceViewFor(powerState(vm))
	} else {
		delete(props, "instanceView")
	}
	osProfile := asMap(props["osProfile"])
	if osProfile == nil {
		return out
	}
	delete(osProfile, "adminPassword")
	linuxConfig := asMap(osProfile["linuxConfiguration"])
	ssh := asMap(linuxConfig["ssh"])
	if ssh != nil {
		delete(ssh, "publicKeys")
	}
	return out
}

func sanitizeExtension(ext map[string]interface{}) map[string]interface{} {
	out := recursiveCopy(ext).(map[string]interface{})
	props := asMap(out["properties"])
	delete(props, "protectedSettings")
	return out
}

func idFromRef(v interface{}) string {
	m := asMap(v)
	id, _ := m["id"].(string)
	return id
}

func (h *Handler) CreateOrUpdate(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "vmName")

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		azerr.BadRequest(w, "The request content was invalid: "+err.Error())
		return
	}
	if input == nil {
		input = map[string]interface{}{}
	}

	k := h.key(sub, rg, name)
	existing, exists := h.store.Get(k)
	vm := buildResponse(sub, rg, name, input, asMap(existing))
	h.store.Set(k, vm)
	h.refreshReferences(sub, vm)
	if exists {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	json.NewEncoder(w).Encode(sanitizeVM(vm, false))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "vmName")

	k := h.key(sub, rg, name)
	existing, ok := h.store.Get(k)
	if !ok {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines", name)
		return
	}
	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		azerr.BadRequest(w, "The request content was invalid: "+err.Error())
		return
	}
	if patch == nil {
		patch = map[string]interface{}{}
	}
	existingMap := asMap(existing)
	vm := buildResponse(sub, rg, name, mergePatch(existingMap, patch), existingMap)
	h.store.Set(k, vm)
	h.refreshReferences(sub, vm)
	json.NewEncoder(w).Encode(sanitizeVM(vm, false))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "vmName")

	v, ok := h.store.Get(h.key(sub, rg, name))
	if !ok {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines", name)
		return
	}
	expand := strings.EqualFold(r.URL.Query().Get("$expand"), "instanceView")
	json.NewEncoder(w).Encode(sanitizeVM(asMap(v), expand))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "vmName")

	k := h.key(sub, rg, name)
	v, ok := h.store.Get(k)
	if !ok {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines", name)
		return
	}
	h.clearReferences(sub, asMap(v))
	h.store.DeleteByPrefix(h.extensionKey(sub, rg, name, ""))
	h.store.Delete(k)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	items := h.sanitizedVMList("vm:" + sub + ":" + rg + ":")
	json.NewEncoder(w).Encode(map[string]interface{}{"value": items})
}

func (h *Handler) ListInSubscription(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	items := h.sanitizedVMList("vm:" + sub + ":")
	json.NewEncoder(w).Encode(map[string]interface{}{"value": items})
}

func (h *Handler) sanitizedVMList(prefix string) []interface{} {
	items := h.store.ListByPrefix(prefix)
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		if vm := asMap(item); vm != nil {
			out = append(out, sanitizeVM(vm, false))
		}
	}
	return out
}

func (h *Handler) InstanceView(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "vmName")
	v, ok := h.store.Get(h.key(sub, rg, name))
	if !ok {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines", name)
		return
	}
	json.NewEncoder(w).Encode(instanceViewFor(powerState(asMap(v))))
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	h.setPowerState(w, r, "PowerState/running")
}

func (h *Handler) PowerOff(w http.ResponseWriter, r *http.Request) {
	h.setPowerState(w, r, "PowerState/stopped")
}

func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	h.setPowerState(w, r, "PowerState/running")
}

func (h *Handler) Deallocate(w http.ResponseWriter, r *http.Request) {
	h.setPowerState(w, r, "PowerState/deallocated")
}

func (h *Handler) Redeploy(w http.ResponseWriter, r *http.Request) {
	h.setPowerState(w, r, "PowerState/running")
}

func (h *Handler) setPowerState(w http.ResponseWriter, r *http.Request, state string) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	name := chi.URLParam(r, "vmName")
	k := h.key(sub, rg, name)
	v, ok := h.store.Get(k)
	if !ok {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines", name)
		return
	}
	vm := asMap(v)
	vm["_powerState"] = state
	h.store.Set(k, vm)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) refreshReferences(sub string, vm map[string]interface{}) {
	h.clearReferences(sub, vm)
	vmID, _ := vm["id"].(string)
	props := asMap(vm["properties"])
	networkProfile := asMap(props["networkProfile"])
	for _, item := range asArray(networkProfile["networkInterfaces"]) {
		nicID := idFromRef(item)
		if nicID == "" {
			continue
		}
		h.setReferenceOnMatchingResource("nic:"+sub+":", nicID, "virtualMachine", map[string]interface{}{"id": vmID})
	}

	for _, diskID := range diskIDsFromStorageProfile(asMap(props["storageProfile"])) {
		h.setReferenceOnMatchingResource("disk:"+sub+":", diskID, "managedBy", vmID)
		h.setReferenceOnMatchingResource("disk:"+sub+":", diskID, "diskState", "Attached")
	}
}

func (h *Handler) clearReferences(sub string, vm map[string]interface{}) {
	vmID, _ := vm["id"].(string)
	for _, item := range h.store.ListByPrefix("nic:" + sub + ":") {
		nic := asMap(item)
		props := asMap(nic["properties"])
		if strings.EqualFold(idFromRef(props["virtualMachine"]), vmID) {
			delete(props, "virtualMachine")
		}
	}
	for _, item := range h.store.ListByPrefix("disk:" + sub + ":") {
		disk := asMap(item)
		props := asMap(disk["properties"])
		if strings.EqualFold(firstString(props["managedBy"]), vmID) {
			props["managedBy"] = ""
			props["diskState"] = "Unattached"
		}
	}
}

func diskIDsFromStorageProfile(storageProfile map[string]interface{}) []string {
	ids := []string{}
	if osDiskID := idFromRef(asMap(storageProfile["osDisk"])["managedDisk"]); osDiskID != "" {
		ids = append(ids, osDiskID)
	}
	for _, item := range asArray(storageProfile["dataDisks"]) {
		disk := asMap(item)
		if diskID := idFromRef(disk["managedDisk"]); diskID != "" {
			ids = append(ids, diskID)
		}
	}
	return ids
}

func (h *Handler) setReferenceOnMatchingResource(prefix, targetID, property string, value interface{}) {
	for _, item := range h.store.ListByPrefix(prefix) {
		resource := asMap(item)
		id, _ := resource["id"].(string)
		if !strings.EqualFold(id, targetID) {
			continue
		}
		props := asMap(resource["properties"])
		props[property] = value
		return
	}
}

func buildExtensionResponse(sub, rg, vm, name string, input map[string]interface{}, existing map[string]interface{}) map[string]interface{} {
	vmResource := asMap(existing["_vm"])
	location := firstString(input["location"], existing["location"], vmResource["location"])
	if location == "" {
		location = "eastus"
	}
	props := asMap(input["properties"])
	if props == nil {
		props = copyMap(asMap(existing["properties"]))
	}
	extensionProps := copyMap(props)
	extensionProps["provisioningState"] = "Succeeded"
	resp := map[string]interface{}{
		"id":         extensionResourceID(sub, rg, vm, name),
		"name":       name,
		"type":       "Microsoft.Compute/virtualMachines/extensions",
		"location":   location,
		"properties": extensionProps,
	}
	if tags := asMap(input["tags"]); tags != nil {
		resp["tags"] = tags
	} else if tags := asMap(existing["tags"]); tags != nil {
		resp["tags"] = tags
	}
	return resp
}

func (h *Handler) CreateOrUpdateExtension(w http.ResponseWriter, r *http.Request) {
	h.writeExtension(w, r, false)
}

func (h *Handler) UpdateExtension(w http.ResponseWriter, r *http.Request) {
	h.writeExtension(w, r, true)
}

func (h *Handler) writeExtension(w http.ResponseWriter, r *http.Request, patch bool) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	vmName := chi.URLParam(r, "vmName")
	name := chi.URLParam(r, "extensionName")
	vm, ok := h.store.Get(h.key(sub, rg, vmName))
	if !ok {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines", vmName)
		return
	}
	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		azerr.BadRequest(w, "The request content was invalid: "+err.Error())
		return
	}
	if input == nil {
		input = map[string]interface{}{}
	}
	k := h.extensionKey(sub, rg, vmName, name)
	existing, exists := h.store.Get(k)
	if patch {
		if !exists {
			azerr.NotFound(w, "Microsoft.Compute/virtualMachines/extensions", name)
			return
		}
		input = mergePatch(asMap(existing), input)
	}
	existingMap := asMap(existing)
	if existingMap == nil {
		existingMap = map[string]interface{}{}
	}
	existingMap["_vm"] = vm
	ext := buildExtensionResponse(sub, rg, vmName, name, input, existingMap)
	h.store.Set(k, ext)
	if !patch && !exists {
		w.WriteHeader(http.StatusCreated)
	}
	json.NewEncoder(w).Encode(sanitizeExtension(ext))
}

func (h *Handler) GetExtension(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	vmName := chi.URLParam(r, "vmName")
	name := chi.URLParam(r, "extensionName")
	v, ok := h.store.Get(h.extensionKey(sub, rg, vmName, name))
	if !ok {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines/extensions", name)
		return
	}
	json.NewEncoder(w).Encode(sanitizeExtension(asMap(v)))
}

func (h *Handler) DeleteExtension(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	vmName := chi.URLParam(r, "vmName")
	name := chi.URLParam(r, "extensionName")
	if !h.store.Delete(h.extensionKey(sub, rg, vmName, name)) {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines/extensions", name)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) ListExtensions(w http.ResponseWriter, r *http.Request) {
	sub := chi.URLParam(r, "subscriptionId")
	rg := chi.URLParam(r, "resourceGroupName")
	vmName := chi.URLParam(r, "vmName")
	if !h.store.Exists(h.key(sub, rg, vmName)) {
		azerr.NotFound(w, "Microsoft.Compute/virtualMachines", vmName)
		return
	}
	items := h.store.ListByPrefix(h.extensionKey(sub, rg, vmName, ""))
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		if ext := asMap(item); ext != nil {
			out = append(out, sanitizeExtension(ext))
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"value": out})
}

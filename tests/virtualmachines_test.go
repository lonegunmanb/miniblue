package tests

import "testing"

func TestVirtualMachineLifecycleReferencesActionsAndSensitiveFields(t *testing.T) {
	ts := setupServer()
	defer ts.Close()

	av := "?api-version=2023-09-01"
	sub := ts.URL + "/subscriptions/sub1"
	net := sub + "/resourceGroups/rg1/providers/Microsoft.Network"
	compute := sub + "/resourceGroups/rg1/providers/Microsoft.Compute"
	nicID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkInterfaces/nic1"
	osDiskID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/osdisk1"
	dataDiskID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/datadisk1"
	vmID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1"

	resp := doRequest(t, "PUT", net+"/networkInterfaces/nic1"+av,
		`{"location":"eastus","properties":{"ipConfigurations":[{"name":"ipconfig1","properties":{"privateIPAddress":"10.0.0.10","privateIPAllocationMethod":"Static"}}]}}`)
	expectStatus(t, resp, 201)
	resp.Body.Close()
	resp = doRequest(t, "PUT", compute+"/disks/osdisk1"+av,
		`{"location":"eastus","properties":{"creationData":{"createOption":"Empty"},"diskSizeGB":32}}`)
	expectStatus(t, resp, 201)
	resp.Body.Close()
	resp = doRequest(t, "PUT", compute+"/disks/datadisk1"+av,
		`{"location":"eastus","properties":{"creationData":{"createOption":"Empty"},"diskSizeGB":64}}`)
	expectStatus(t, resp, 201)
	resp.Body.Close()

	vmBody := `{
		"location":"eastus",
		"tags":{"env":"test"},
		"properties":{
			"hardwareProfile":{"vmSize":"Standard_B1s"},
			"storageProfile":{
				"imageReference":{"publisher":"Canonical","offer":"0001-com-ubuntu-server-jammy","sku":"22_04-lts","version":"latest"},
				"osDisk":{"name":"osdisk1","createOption":"Attach","managedDisk":{"id":"` + osDiskID + `"}},
				"dataDisks":[{"lun":0,"name":"datadisk1","createOption":"Attach","managedDisk":{"id":"` + dataDiskID + `"}}]
			},
			"osProfile":{
				"computerName":"vm1",
				"adminUsername":"azureuser",
				"adminPassword":"super-secret",
				"linuxConfiguration":{"ssh":{"publicKeys":[{"path":"/home/azureuser/.ssh/authorized_keys","keyData":"ssh-rsa AAA"}]}}
			},
			"networkProfile":{"networkInterfaces":[{"id":"` + nicID + `","properties":{"primary":true}}]}
		}
	}`
	resp = doRequest(t, "PUT", compute+"/virtualMachines/vm1"+av, vmBody)
	expectStatus(t, resp, 201)
	vm := decodeJSON(t, resp)
	resp.Body.Close()
	if vm["id"] != vmID {
		t.Fatalf("expected VM id %s, got %v", vmID, vm["id"])
	}
	assertVMResponseShape(t, vm)

	resp = doRequest(t, "GET", compute+"/virtualMachines/vm1"+av, "")
	expectStatus(t, resp, 200)
	vm = decodeJSON(t, resp)
	resp.Body.Close()
	assertVMResponseShape(t, vm)

	resp = doRequest(t, "GET", net+"/networkInterfaces/nic1"+av, "")
	expectStatus(t, resp, 200)
	nic := decodeJSON(t, resp)
	resp.Body.Close()
	if got := nic["properties"].(map[string]interface{})["virtualMachine"].(map[string]interface{})["id"]; got != vmID {
		t.Fatalf("expected NIC virtualMachine reference %s, got %v", vmID, got)
	}

	for _, diskName := range []string{"osdisk1", "datadisk1"} {
		resp = doRequest(t, "GET", compute+"/disks/"+diskName+av, "")
		expectStatus(t, resp, 200)
		disk := decodeJSON(t, resp)
		resp.Body.Close()
		props := disk["properties"].(map[string]interface{})
		if props["managedBy"] != vmID || props["diskState"] != "Attached" {
			t.Fatalf("expected %s to be attached to VM, got managedBy=%v state=%v", diskName, props["managedBy"], props["diskState"])
		}
	}

	resp = doRequest(t, "PATCH", compute+"/virtualMachines/vm1"+av,
		`{"tags":{"patched":"true"},"properties":{"hardwareProfile":{"vmSize":"Standard_B2s"}}}`)
	expectStatus(t, resp, 200)
	patched := decodeJSON(t, resp)
	resp.Body.Close()
	if patched["tags"].(map[string]interface{})["patched"] != "true" {
		t.Fatalf("expected patched tags")
	}
	if patched["properties"].(map[string]interface{})["hardwareProfile"].(map[string]interface{})["vmSize"] != "Standard_B2s" {
		t.Fatalf("expected patched vmSize")
	}
	assertVMResponseShape(t, patched)

	resp = doRequest(t, "POST", compute+"/virtualMachines/vm1/powerOff"+av, `{}`)
	expectStatus(t, resp, 202)
	resp.Body.Close()
	resp = doRequest(t, "GET", compute+"/virtualMachines/vm1/instanceView"+av, "")
	expectStatus(t, resp, 200)
	instanceView := decodeJSON(t, resp)
	resp.Body.Close()
	statuses := instanceView["statuses"].([]interface{})
	if statuses[len(statuses)-1].(map[string]interface{})["code"] != "PowerState/stopped" {
		t.Fatalf("expected powerOff to set PowerState/stopped, got %v", statuses)
	}
	resp = doRequest(t, "POST", compute+"/virtualMachines/vm1/start"+av, `{}`)
	expectStatus(t, resp, 202)
	resp.Body.Close()
	resp = doRequest(t, "POST", compute+"/virtualMachines/vm1/restart"+av, `{}`)
	expectStatus(t, resp, 202)
	resp.Body.Close()
	resp = doRequest(t, "POST", compute+"/virtualMachines/vm1/deallocate"+av, `{}`)
	expectStatus(t, resp, 202)
	resp.Body.Close()
	resp = doRequest(t, "POST", compute+"/virtualMachines/vm1/redeploy"+av, `{}`)
	expectStatus(t, resp, 202)
	resp.Body.Close()

	resp = doRequest(t, "PUT", compute+"/virtualMachines/vm1/extensions/customScript"+av,
		`{"location":"eastus","properties":{"publisher":"Microsoft.Azure.Extensions","type":"CustomScript","typeHandlerVersion":"2.1","settings":{"commandToExecute":"echo ok"}}}`)
	expectStatus(t, resp, 201)
	resp.Body.Close()
	resp = doRequest(t, "GET", compute+"/virtualMachines/vm1/extensions"+av, "")
	expectStatus(t, resp, 200)
	extensions := decodeJSON(t, resp)
	resp.Body.Close()
	if len(extensions["value"].([]interface{})) != 1 {
		t.Fatalf("expected one VM extension")
	}

	resp = doRequest(t, "GET", compute+"/virtualMachines"+av, "")
	expectStatus(t, resp, 200)
	rgList := decodeJSON(t, resp)
	resp.Body.Close()
	if len(rgList["value"].([]interface{})) != 1 {
		t.Fatalf("expected one VM in resource group")
	}
	resp = doRequest(t, "GET", sub+"/providers/Microsoft.Compute/virtualMachines"+av, "")
	expectStatus(t, resp, 200)
	subList := decodeJSON(t, resp)
	resp.Body.Close()
	if len(subList["value"].([]interface{})) != 1 {
		t.Fatalf("expected one VM in subscription")
	}

	resp = doRequest(t, "DELETE", compute+"/virtualMachines/vm1"+av, "")
	expectStatus(t, resp, 202)
	resp.Body.Close()
	resp = doRequest(t, "GET", compute+"/virtualMachines/vm1"+av, "")
	expectStatus(t, resp, 404)
	resp.Body.Close()
	resp = doRequest(t, "GET", net+"/networkInterfaces/nic1"+av, "")
	expectStatus(t, resp, 200)
	nic = decodeJSON(t, resp)
	resp.Body.Close()
	if _, ok := nic["properties"].(map[string]interface{})["virtualMachine"]; ok {
		t.Fatalf("expected VM delete to clear NIC virtualMachine reference")
	}
	resp = doRequest(t, "GET", compute+"/disks/datadisk1"+av, "")
	expectStatus(t, resp, 200)
	disk := decodeJSON(t, resp)
	resp.Body.Close()
	props := disk["properties"].(map[string]interface{})
	managedBy, _ := props["managedBy"].(string)
	if managedBy != "" || props["diskState"] != "Unattached" {
		t.Fatalf("expected VM delete to detach disk, got managedBy=%v state=%v", props["managedBy"], props["diskState"])
	}
}

func assertVMResponseShape(t *testing.T, vm map[string]interface{}) {
	t.Helper()
	props := vm["properties"].(map[string]interface{})
	osProfile := props["osProfile"].(map[string]interface{})
	if _, ok := osProfile["adminPassword"]; ok {
		t.Fatalf("GET-style VM response must not include adminPassword")
	}
	linuxConfig := osProfile["linuxConfiguration"].(map[string]interface{})
	ssh := linuxConfig["ssh"].(map[string]interface{})
	publicKeys, ok := ssh["publicKeys"].([]interface{})
	if !ok || len(publicKeys) != 1 {
		t.Fatalf("GET-style VM response must include ssh publicKeys, got %v", ssh["publicKeys"])
	}
	publicKey := publicKeys[0].(map[string]interface{})
	if publicKey["path"] != "/home/azureuser/.ssh/authorized_keys" || publicKey["keyData"] != "ssh-rsa AAA" {
		t.Fatalf("unexpected ssh publicKeys payload: %v", publicKey)
	}
}

func TestVirtualMachineFromImageDefaults(t *testing.T) {
	ts := setupServer()
	defer ts.Close()

	av := "?api-version=2023-09-01"
	compute := ts.URL + "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute"

	vmBody := `{
		"location":"eastus",
		"properties":{
			"additionalCapabilities":{},
			"storageProfile":{
				"imageReference":{"publisher":"Canonical","offer":"0001-com-ubuntu-server-jammy","sku":"22_04-lts","version":"latest"},
				"osDisk":{"createOption":"FromImage"}
			},
			"osProfile":{
				"computerName":"vm-defaults",
				"adminUsername":"azureuser",
				"linuxConfiguration":{"ssh":{"publicKeys":[{"path":"/home/azureuser/.ssh/authorized_keys","keyData":"ssh-rsa AAA"}]}}
			}
		}
	}`
	resp := doRequest(t, "PUT", compute+"/virtualMachines/vm-defaults"+av, vmBody)
	expectStatus(t, resp, 201)
	vm := decodeJSON(t, resp)
	resp.Body.Close()
	assertVMResponseShape(t, vm)

	props := vm["properties"].(map[string]interface{})
	storageProfile := props["storageProfile"].(map[string]interface{})
	osDisk := storageProfile["osDisk"].(map[string]interface{})

	if osDisk["diskSizeGB"] != float64(30) {
		t.Fatalf("expected default Linux osDisk.diskSizeGB to be 30, got %v", osDisk["diskSizeGB"])
	}
	osDiskName, _ := osDisk["name"].(string)
	if osDiskName == "" {
		t.Fatalf("expected osDisk.name to be generated")
	}
	managedDisk := osDisk["managedDisk"].(map[string]interface{})
	expectedManagedDiskID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/" + osDiskName
	if managedDisk["id"] != expectedManagedDiskID {
		t.Fatalf("expected generated managedDisk id %s, got %v", expectedManagedDiskID, managedDisk["id"])
	}
	if _, ok := props["vmId"].(string); !ok || props["vmId"] == "" {
		t.Fatalf("expected properties.vmId to be generated, got %v", props["vmId"])
	}
	additionalCapabilities := props["additionalCapabilities"].(map[string]interface{})
	if additionalCapabilities["hibernationEnabled"] != false || additionalCapabilities["ultraSSDEnabled"] != false {
		t.Fatalf("expected additionalCapabilities defaults to be false/false, got %v", additionalCapabilities)
	}
}

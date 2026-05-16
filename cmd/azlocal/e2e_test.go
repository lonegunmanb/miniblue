package main

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/moabukar/miniblue/internal/server"
)

func setupMiniblue() *httptest.Server {
	srv := server.New()
	return httptest.NewServer(srv.Handler())
}

func runAzlocal(ts *httptest.Server, args ...string) (string, string, int) {
	cwd, _ := os.Getwd()
	binPath := cwd + "/../../bin/azlocal"
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), "LOCAL_AZURE_ENDPOINT="+ts.URL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err.Error(), -1
	}
	return string(output), "", 0
}

func TestStorageAccountCreate(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	output, _, _ := runAzlocal(ts, "storage", "account", "create",
		"--resource-group", "myRG",
		"--name", "testacct")

	if !strings.Contains(output, "name") || !strings.Contains(output, "testacct") {
		t.Fatalf("expected account name in output, got: %s", output)
	}
}

func TestStorageAccountCreateWithFlags(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	output, _, _ := runAzlocal(ts, "storage", "account", "create",
		"--resource-group", "myRG",
		"--name", "testacct2",
		"--location", "westus2",
		"--sku", "Premium_LRS")

	if !strings.Contains(output, "name") {
		t.Fatalf("expected account response, got: %s", output)
	}
}

func TestStorageAccountList(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "storage", "account", "create", "--resource-group", "myRG", "--name", "acct1")
	runAzlocal(ts, "storage", "account", "create", "--resource-group", "myRG", "--name", "acct2")

	output, _, _ := runAzlocal(ts, "storage", "account", "list", "--resource-group", "myRG")

	if !strings.Contains(output, "acct1") || !strings.Contains(output, "acct2") {
		t.Fatalf("expected both accounts in list, got: %s", output)
	}
}

func TestStorageAccountShow(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "storage", "account", "create", "--resource-group", "myRG", "--name", "showacct")

	output, _, _ := runAzlocal(ts, "storage", "account", "show",
		"--resource-group", "myRG",
		"--name", "showacct")

	if !strings.Contains(output, "showacct") || !strings.Contains(output, "Microsoft.Storage/storageAccounts") {
		t.Fatalf("expected account details in output, got: %s", output)
	}
}

func TestStorageAccountShowNotFound(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	output, _, _ := runAzlocal(ts, "storage", "account", "show",
		"--resource-group", "myRG",
		"--name", "nonexistent")

	if !strings.Contains(output, "404") && !strings.Contains(output, "NotFound") {
		t.Fatalf("expected not found error in output, got: %s", output)
	}
}

func TestStorageAccountListKeys(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "storage", "account", "create", "--resource-group", "myRG", "--name", "keyacct")

	output, _, _ := runAzlocal(ts, "storage", "account", "list-keys",
		"--resource-group", "myRG",
		"--name", "keyacct")

	if !strings.Contains(output, "key1") || !strings.Contains(output, "key2") {
		t.Fatalf("expected keys in output, got: %s", output)
	}
}

func TestStorageAccountDelete(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "storage", "account", "create", "--resource-group", "myRG", "--name", "deleteacct")

	output, _, _ := runAzlocal(ts, "storage", "account", "delete",
		"--resource-group", "myRG",
		"--name", "deleteacct")

	if !strings.Contains(strings.ToLower(output), "deleted") {
		t.Fatalf("expected delete confirmation, got: %s", output)
	}

	showOutput, _, _ := runAzlocal(ts, "storage", "account", "show",
		"--resource-group", "myRG",
		"--name", "deleteacct")

	if !strings.Contains(showOutput, "404") && !strings.Contains(showOutput, "NotFound") {
		t.Fatalf("expected not found error after deletion, got: %s", showOutput)
	}
}

func TestStorageAccountMissingResourceGroup(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	_, _, code := runAzlocal(ts, "storage", "account", "create",
		"--name", "testacct")

	if code == 0 {
		t.Fatal("expected error for missing --resource-group")
	}
}

func TestKeyVaultSecretCommands(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	setOut, _, code := runAzlocal(ts, "keyvault", "secret", "set",
		"--vault", "myvault",
		"--name", "db-pass",
		"--value", "supersecret")
	if code != 0 {
		t.Fatalf("keyvault secret set failed: %s", setOut)
	}
	if !strings.Contains(setOut, "supersecret") || !strings.Contains(setOut, "https://myvault.vault.azure.net/secrets/db-pass/") {
		t.Fatalf("expected secret value and versioned id in set output, got: %s", setOut)
	}

	showOut, _, code := runAzlocal(ts, "keyvault", "secret", "show",
		"--vault", "myvault",
		"--name", "db-pass")
	if code != 0 {
		t.Fatalf("keyvault secret show failed: %s", showOut)
	}
	if !strings.Contains(showOut, "supersecret") {
		t.Fatalf("expected secret value in show output, got: %s", showOut)
	}

	listOut, _, code := runAzlocal(ts, "keyvault", "secret", "list",
		"--vault", "myvault")
	if code != 0 {
		t.Fatalf("keyvault secret list failed: %s", listOut)
	}
	if !strings.Contains(listOut, "db-pass") || strings.Contains(listOut, "supersecret") {
		t.Fatalf("expected redacted secret metadata in list output, got: %s", listOut)
	}

	deleteOut, _, code := runAzlocal(ts, "keyvault", "secret", "delete",
		"--vault", "myvault",
		"--name", "db-pass")
	if code != 0 {
		t.Fatalf("keyvault secret delete failed: %s", deleteOut)
	}
	if !strings.Contains(strings.ToLower(deleteOut), "deleted") {
		t.Fatalf("expected delete confirmation, got: %s", deleteOut)
	}
}

func TestIdentityCreateShowUpdateListDelete(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	createOut, _, code := runAzlocal(ts, "identity", "create",
		"--resource-group", "myRG",
		"--name", "id1",
		"--location", "westus2",
		"--tags", "env=test")
	if code != 0 {
		t.Fatalf("identity create failed: %s", createOut)
	}
	if !strings.Contains(createOut, "\"name\": \"id1\"") ||
		!strings.Contains(createOut, "Microsoft.ManagedIdentity/userAssignedIdentities") ||
		!strings.Contains(createOut, "principalId") ||
		!strings.Contains(createOut, "clientId") ||
		!strings.Contains(createOut, "tenantId") {
		t.Fatalf("expected identity details in create output, got: %s", createOut)
	}

	showOut, _, _ := runAzlocal(ts, "identity", "show",
		"--resource-group", "myRG",
		"--name", "id1")
	if !strings.Contains(showOut, "\"name\": \"id1\"") {
		t.Fatalf("expected identity name in show output, got: %s", showOut)
	}

	updateOut, _, code := runAzlocal(ts, "identity", "update",
		"--resource-group", "myRG",
		"--name", "id1",
		"--tags", "env=patched")
	if code != 0 {
		t.Fatalf("identity update failed: %s", updateOut)
	}
	if !strings.Contains(updateOut, "\"env\": \"patched\"") {
		t.Fatalf("expected patched tag, got: %s", updateOut)
	}

	listOut, _, _ := runAzlocal(ts, "identity", "list", "--resource-group", "myRG")
	if !strings.Contains(listOut, "\"name\": \"id1\"") {
		t.Fatalf("expected identity in list output, got: %s", listOut)
	}

	deleteOut, _, _ := runAzlocal(ts, "identity", "delete",
		"--resource-group", "myRG",
		"--name", "id1")
	if !strings.Contains(strings.ToLower(deleteOut), "deleted") {
		t.Fatalf("expected delete confirmation, got: %s", deleteOut)
	}
}

func TestRoleDefinitionListAndShow(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	listOut, _, code := runAzlocal(ts, "role", "definition", "list",
		"--name", "Storage Blob Data Contributor")
	if code != 0 {
		t.Fatalf("role definition list failed: %s", listOut)
	}
	if !strings.Contains(listOut, "Storage Blob Data Contributor") ||
		!strings.Contains(listOut, "ba92f5b4-2d11-453d-a403-e96b0029c9fe") {
		t.Fatalf("expected filtered built-in role in list output, got: %s", listOut)
	}

	showOut, _, code := runAzlocal(ts, "role", "definition", "show",
		"--name", "Reader")
	if code != 0 {
		t.Fatalf("role definition show failed: %s", showOut)
	}
	if !strings.Contains(showOut, "\"roleName\": \"Reader\"") ||
		!strings.Contains(showOut, "Microsoft.Authorization/roleDefinitions") {
		t.Fatalf("expected Reader role definition in show output, got: %s", showOut)
	}
}

func TestRoleAssignmentCreateListDelete(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	scope := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myRG"
	name := "11111111-1111-1111-1111-111111111111"
	createOut, _, code := runAzlocal(ts, "role", "assignment", "create",
		"--scope", scope,
		"--name", name,
		"--assignee", "principal-1",
		"--role", "Reader")
	if code != 0 {
		t.Fatalf("role assignment create failed: %s", createOut)
	}
	if !strings.Contains(createOut, "\"principalId\": \"principal-1\"") ||
		!strings.Contains(createOut, "acdd72a7-3385-48ef-bd42-f606fba81ae7") ||
		!strings.Contains(createOut, scope) {
		t.Fatalf("expected role assignment details in create output, got: %s", createOut)
	}

	listOut, _, code := runAzlocal(ts, "role", "assignment", "list", "--scope", scope)
	if code != 0 {
		t.Fatalf("role assignment list failed: %s", listOut)
	}
	if !strings.Contains(listOut, name) || !strings.Contains(listOut, "principal-1") {
		t.Fatalf("expected role assignment in list output, got: %s", listOut)
	}

	deleteOut, _, code := runAzlocal(ts, "role", "assignment", "delete",
		"--scope", scope,
		"--name", name)
	if code != 0 {
		t.Fatalf("role assignment delete failed: %s", deleteOut)
	}
	if !strings.Contains(strings.ToLower(deleteOut), "deleted") {
		t.Fatalf("expected delete confirmation, got: %s", deleteOut)
	}
}

func TestCosmosDBTableCommands(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	createOut, _, code := runAzlocal(ts, "cosmosdb", "table", "create",
		"--resource-group", "myRG",
		"--account", "acct1",
		"--name", "users",
		"--data", `{"properties":{"resource":{"id":"users"}}}`)
	if code != 0 {
		t.Fatalf("cosmosdb table create failed: %s", createOut)
	}
	if !strings.Contains(createOut, "\"name\": \"users\"") ||
		!strings.Contains(createOut, "Microsoft.DocumentDB/databaseAccounts/tables") {
		t.Fatalf("expected table details in create output, got: %s", createOut)
	}

	listOut, _, code := runAzlocal(ts, "cosmosdb", "table", "list",
		"--resource-group", "myRG",
		"--account", "acct1")
	if code != 0 {
		t.Fatalf("cosmosdb table list failed: %s", listOut)
	}
	if !strings.Contains(listOut, "\"name\": \"users\"") {
		t.Fatalf("expected table in list output, got: %s", listOut)
	}

	throughputOut, _, code := runAzlocal(ts, "cosmosdb", "table", "throughput", "update",
		"--resource-group", "myRG",
		"--account", "acct1",
		"--name", "users",
		"--data", `{"properties":{"resource":{"throughput":400}}}`)
	if code != 0 {
		t.Fatalf("cosmosdb table throughput update failed: %s", throughputOut)
	}
	if !strings.Contains(throughputOut, "\"name\": \"default\"") ||
		!strings.Contains(throughputOut, "\"throughput\": 400") {
		t.Fatalf("expected throughput details in update output, got: %s", throughputOut)
	}

	showThroughputOut, _, code := runAzlocal(ts, "cosmosdb", "table", "throughput", "show",
		"--resource-group", "myRG",
		"--account", "acct1",
		"--name", "users")
	if code != 0 {
		t.Fatalf("cosmosdb table throughput show failed: %s", showThroughputOut)
	}
	if !strings.Contains(showThroughputOut, "\"throughput\": 400") {
		t.Fatalf("expected throughput in show output, got: %s", showThroughputOut)
	}

	deleteOut, _, code := runAzlocal(ts, "cosmosdb", "table", "delete",
		"--resource-group", "myRG",
		"--account", "acct1",
		"--name", "users")
	if code != 0 {
		t.Fatalf("cosmosdb table delete failed: %s", deleteOut)
	}
	if !strings.Contains(strings.ToLower(deleteOut), "deleted") {
		t.Fatalf("expected delete confirmation, got: %s", deleteOut)
	}
}

// --- vm ---

func TestVMCreateShowListDelete(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	createOut, _, code := runAzlocal(ts, "vm", "create",
		"--resource-group", "myRG",
		"--name", "vm1",
		"--location", "westus2",
		"--size", "Standard_B1s",
		"--image", "Ubuntu2204",
		"--admin-username", "azureuser")
	if code != 0 {
		t.Fatalf("vm create failed: %s", createOut)
	}
	if !strings.Contains(createOut, "\"name\": \"vm1\"") ||
		!strings.Contains(createOut, "Microsoft.Compute/virtualMachines") ||
		!strings.Contains(createOut, "Standard_B1s") ||
		!strings.Contains(createOut, "azureuser") {
		t.Fatalf("expected VM details in create output, got: %s", createOut)
	}

	showOut, _, _ := runAzlocal(ts, "vm", "show",
		"--resource-group", "myRG",
		"--name", "vm1")
	if !strings.Contains(showOut, "\"name\": \"vm1\"") {
		t.Fatalf("expected VM name in show output, got: %s", showOut)
	}

	listOut, _, _ := runAzlocal(ts, "vm", "list", "--resource-group", "myRG")
	if !strings.Contains(listOut, "\"name\": \"vm1\"") {
		t.Fatalf("expected VM in list output, got: %s", listOut)
	}

	deleteOut, _, _ := runAzlocal(ts, "vm", "delete",
		"--resource-group", "myRG",
		"--name", "vm1")
	if !strings.Contains(strings.ToLower(deleteOut), "deleted") {
		t.Fatalf("expected delete confirmation, got: %s", deleteOut)
	}
}

func TestVMPowerActionsAndInstanceView(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "vm", "create", "--resource-group", "myRG", "--name", "vm1")

	stopOut, _, code := runAzlocal(ts, "vm", "stop", "--resource-group", "myRG", "--name", "vm1")
	if code != 0 {
		t.Fatalf("vm stop failed: %s", stopOut)
	}

	instanceViewOut, _, _ := runAzlocal(ts, "vm", "get-instance-view",
		"--resource-group", "myRG",
		"--name", "vm1")
	if !strings.Contains(instanceViewOut, "PowerState/stopped") {
		t.Fatalf("expected stopped power state, got: %s", instanceViewOut)
	}

	startOut, _, code := runAzlocal(ts, "vm", "start", "--resource-group", "myRG", "--name", "vm1")
	if code != 0 {
		t.Fatalf("vm start failed: %s", startOut)
	}
}

func TestVMExtensionLifecycle(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "vm", "create", "--resource-group", "myRG", "--name", "vm1")

	setOut, _, code := runAzlocal(ts, "vm", "extension", "set",
		"--resource-group", "myRG",
		"--vm-name", "vm1",
		"--name", "customScript",
		"--publisher", "Microsoft.Azure.Extensions",
		"--type", "CustomScript",
		"--settings", `{"commandToExecute":"echo ok"}`)
	if code != 0 {
		t.Fatalf("vm extension set failed: %s", setOut)
	}
	if !strings.Contains(setOut, "\"name\": \"customScript\"") {
		t.Fatalf("expected extension name in set output, got: %s", setOut)
	}

	listOut, _, _ := runAzlocal(ts, "vm", "extension", "list",
		"--resource-group", "myRG",
		"--vm-name", "vm1")
	if !strings.Contains(listOut, "customScript") {
		t.Fatalf("expected extension in list output, got: %s", listOut)
	}

	deleteOut, _, _ := runAzlocal(ts, "vm", "extension", "delete",
		"--resource-group", "myRG",
		"--vm-name", "vm1",
		"--name", "customScript")
	if !strings.Contains(strings.ToLower(deleteOut), "deleted") {
		t.Fatalf("expected extension delete confirmation, got: %s", deleteOut)
	}
}

// --- network vnet subnet ---

func TestNetworkVNetSubnetCreateAndShow(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "network", "vnet", "create",
		"--resource-group", "myRG", "--name", "vnet1",
		"--address-prefix", "10.0.0.0/16")

	createOut, _, code := runAzlocal(ts, "network", "vnet", "subnet", "create",
		"--resource-group", "myRG", "--vnet-name", "vnet1",
		"--name", "web", "--address-prefixes", "10.0.1.0/24")
	if code != 0 {
		t.Fatalf("subnet create failed: %s", createOut)
	}
	if !strings.Contains(createOut, "\"name\": \"web\"") || !strings.Contains(createOut, "10.0.1.0/24") {
		t.Fatalf("expected subnet name and prefix in create output, got: %s", createOut)
	}

	showOut, _, _ := runAzlocal(ts, "network", "vnet", "subnet", "show",
		"--resource-group", "myRG", "--vnet-name", "vnet1", "--name", "web")
	if !strings.Contains(showOut, "Microsoft.Network/virtualNetworks/subnets") {
		t.Fatalf("expected subnet type in show output, got: %s", showOut)
	}
}

func TestNetworkVNetSubnetList(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "network", "vnet", "create",
		"--resource-group", "myRG", "--name", "vnet1",
		"--address-prefix", "10.0.0.0/16")
	runAzlocal(ts, "network", "vnet", "subnet", "create",
		"--resource-group", "myRG", "--vnet-name", "vnet1",
		"--name", "web", "--address-prefixes", "10.0.1.0/24")
	runAzlocal(ts, "network", "vnet", "subnet", "create",
		"--resource-group", "myRG", "--vnet-name", "vnet1",
		"--name", "app", "--address-prefixes", "10.0.2.0/24")

	out, _, _ := runAzlocal(ts, "network", "vnet", "subnet", "list",
		"--resource-group", "myRG", "--vnet-name", "vnet1")
	if !strings.Contains(out, "web") || !strings.Contains(out, "app") {
		t.Fatalf("expected both subnets in list, got: %s", out)
	}
}

func TestNetworkVNetSubnetUpdate(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "network", "vnet", "create",
		"--resource-group", "myRG", "--name", "vnet1",
		"--address-prefix", "10.0.0.0/16")
	runAzlocal(ts, "network", "vnet", "subnet", "create",
		"--resource-group", "myRG", "--vnet-name", "vnet1",
		"--name", "web", "--address-prefixes", "10.0.1.0/24")

	out, _, code := runAzlocal(ts, "network", "vnet", "subnet", "update",
		"--resource-group", "myRG", "--vnet-name", "vnet1",
		"--name", "web", "--address-prefixes", "10.0.5.0/24")
	if code != 0 {
		t.Fatalf("subnet update failed: %s", out)
	}
	if !strings.Contains(out, "10.0.5.0/24") {
		t.Fatalf("expected updated prefix in output, got: %s", out)
	}
}

func TestNetworkVNetSubnetDelete(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	runAzlocal(ts, "network", "vnet", "create",
		"--resource-group", "myRG", "--name", "vnet1",
		"--address-prefix", "10.0.0.0/16")
	runAzlocal(ts, "network", "vnet", "subnet", "create",
		"--resource-group", "myRG", "--vnet-name", "vnet1",
		"--name", "web", "--address-prefixes", "10.0.1.0/24")

	out, _, _ := runAzlocal(ts, "network", "vnet", "subnet", "delete",
		"--resource-group", "myRG", "--vnet-name", "vnet1", "--name", "web")
	if !strings.Contains(strings.ToLower(out), "deleted") {
		t.Fatalf("expected delete confirmation, got: %s", out)
	}

	showOut, _, _ := runAzlocal(ts, "network", "vnet", "subnet", "show",
		"--resource-group", "myRG", "--vnet-name", "vnet1", "--name", "web")
	if !strings.Contains(showOut, "404") && !strings.Contains(showOut, "NotFound") {
		t.Fatalf("expected not-found after delete, got: %s", showOut)
	}
}

func TestNetworkVNetSubnetUnknownAction(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	out, _, code := runAzlocal(ts, "network", "vnet", "subnet", "frobnicate",
		"--resource-group", "myRG", "--vnet-name", "vnet1")
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown subnet action, got 0; output: %s", out)
	}
	if !strings.Contains(out, "Unknown subcommand") {
		t.Fatalf("expected 'Unknown subcommand' in output, got: %s", out)
	}
}

func TestNetworkVNetUnknownAction(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	out, _, code := runAzlocal(ts, "network", "vnet", "frobnicate",
		"--resource-group", "myRG", "--name", "vnet1")
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown vnet action, got 0; output: %s", out)
	}
	if !strings.Contains(out, "Unknown subcommand") {
		t.Fatalf("expected 'Unknown subcommand' in output, got: %s", out)
	}
}

func TestNetworkUnknownResource(t *testing.T) {
	ts := setupMiniblue()
	defer ts.Close()

	out, _, code := runAzlocal(ts, "network", "frobnicate", "list")
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown network resource, got 0; output: %s", out)
	}
	if !strings.Contains(out, "Unknown subcommand") {
		t.Fatalf("expected 'Unknown subcommand' in output, got: %s", out)
	}
}

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

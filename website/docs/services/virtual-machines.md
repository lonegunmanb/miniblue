# Virtual Machines

miniblue emulates `Microsoft.Compute/virtualMachines` and the surrounding
compute catalog endpoints (VM sizes, resource SKUs, marketplace images) used
by Terraform's `azurerm` provider, the Azure CLI and the Azure SDKs.

VMs are stored in memory (or persisted via `PERSISTENCE=1`/`DATABASE_URL`) and
no real hypervisor is involved — power operations simply mutate the persisted
power state and instance-view fields.

## ARM endpoints

### Virtual machines

| Method | Path | Description |
|--------|------|-------------|
| `PUT` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}` | Create or replace |
| `PATCH` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}` | Update tags / properties |
| `GET` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}` | Get (supports `?$expand=instanceView`) |
| `DELETE` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}` | Delete |
| `GET` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines` | List in resource group |
| `GET` | `/subscriptions/{sub}/providers/Microsoft.Compute/virtualMachines` | List in subscription |
| `GET` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/virtualMachines/{name}/instanceView` | Instance view |
| `POST` | `.../virtualMachines/{name}/start` | Start |
| `POST` | `.../virtualMachines/{name}/powerOff` | Power off |
| `POST` | `.../virtualMachines/{name}/restart` | Restart |
| `POST` | `.../virtualMachines/{name}/deallocate` | Deallocate |
| `POST` | `.../virtualMachines/{name}/redeploy` | Redeploy |

### VM extensions

| Method | Path | Description |
|--------|------|-------------|
| `PUT` / `PATCH` / `GET` / `DELETE` | `.../virtualMachines/{name}/extensions/{extensionName}` | Per-extension CRUD |
| `GET` | `.../virtualMachines/{name}/extensions` | List extensions |

### Compute catalog (read-only)

These mirror the Azure catalog endpoints that Terraform calls during plan to
resolve VM sizes and marketplace images:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/subscriptions/{sub}/providers/Microsoft.Compute/locations/{location}/vmSizes` | List VM sizes for a location |
| `GET` | `/subscriptions/{sub}/providers/Microsoft.Compute/skus` | List resource SKUs |
| `GET` | `/subscriptions/{sub}/providers/Microsoft.Compute/locations/{location}/publishers` | List image publishers |
| `GET` | `.../publishers/{publisher}/artifacttypes/vmimage/offers` | List offers |
| `GET` | `.../offers/{offer}/skus` | List image SKUs |
| `GET` | `.../skus/{sku}/versions` | List image versions |
| `GET` | `.../versions/{version}` | Get a specific image version |

The catalog returns a small, static set of well-known SKUs and the common
Ubuntu / Windows Server publishers — enough for `azurerm` to plan and apply,
but not a faithful clone of the real Azure catalog.

## Create a VM

```bash
curl -X PUT \
  "http://localhost:4566/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myRG/providers/Microsoft.Compute/virtualMachines/myvm?api-version=2024-07-01" \
  -H "Content-Type: application/json" \
  -d '{
    "location": "eastus",
    "properties": {
      "hardwareProfile": { "vmSize": "Standard_B1s" },
      "storageProfile": {
        "imageReference": {
          "publisher": "Canonical",
          "offer": "0001-com-ubuntu-server-jammy",
          "sku": "22_04-lts",
          "version": "latest"
        }
      },
      "osProfile": {
        "computerName": "myvm",
        "adminUsername": "azureuser",
        "adminPassword": "P@ssw0rd1234!"
      },
      "networkProfile": {
        "networkInterfaces": [
          { "id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myRG/providers/Microsoft.Network/networkInterfaces/mynic" }
        ]
      }
    }
  }'
```

A synthesized OS disk is created in the disk store (see
[Managed Disks](managed-disks.md)) so that subsequent `GET` calls return a
realistic `storageProfile.osDisk.managedDisk.id` reference.

## Power operations

```bash
BASE="http://localhost:4566/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myRG/providers/Microsoft.Compute/virtualMachines/myvm"

curl -X POST "$BASE/powerOff"
curl -X POST "$BASE/start"
curl -X POST "$BASE/restart"
curl -X POST "$BASE/deallocate"
curl "$BASE/instanceView"
```

The instance view reflects the most recent power transition
(`PowerState/running`, `PowerState/stopped`, `PowerState/deallocated`, etc.).

## Terraform

```hcl
resource "azurerm_linux_virtual_machine" "example" {
  name                = "myvm"
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  size                = "Standard_B1s"
  admin_username      = "azureuser"

  network_interface_ids = [azurerm_network_interface.example.id]

  admin_ssh_key {
    username   = "azureuser"
    public_key = file("~/.ssh/id_rsa.pub")
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts"
    version   = "latest"
  }
}
```

## azlocal

```bash
# Create a VM
azlocal vm create --resource-group myRG --name myvm \
  --image Ubuntu2204 --size Standard_B1s \
  --admin-username azureuser --nic mynic

azlocal vm list --resource-group myRG
azlocal vm show --resource-group myRG --name myvm
azlocal vm show --resource-group myRG --name myvm --show-details   # expand=instanceView

# Power operations
azlocal vm stop       --resource-group myRG --name myvm
azlocal vm start      --resource-group myRG --name myvm
azlocal vm restart    --resource-group myRG --name myvm
azlocal vm deallocate --resource-group myRG --name myvm
azlocal vm get-instance-view --resource-group myRG --name myvm

azlocal vm delete --resource-group myRG --name myvm

# Extensions
azlocal vm extension set --resource-group myRG --vm-name myvm \
  --name customScript --publisher Microsoft.Azure.Extensions \
  --type CustomScript --settings '{"commandToExecute":"echo ok"}'
azlocal vm extension list --resource-group myRG --vm-name myvm
azlocal vm extension delete --resource-group myRG --vm-name myvm --name customScript
```

## Limitations

- No real compute — VMs do not boot, run code, or expose network connectivity.
- No remote desktop, serial console, or run-command execution.
- Boot diagnostics, identity assignment and disk encryption properties are
  echoed back on `GET` but not enforced.
- The compute catalog returns a small, static catalog rather than the full
  Azure inventory.

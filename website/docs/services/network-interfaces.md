# Network Interfaces

miniblue emulates Azure Network Interfaces (NICs) via ARM endpoints. NICs support IP configurations with subnet and Public IP references, Network Security Group association, and VM back-references.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `PUT` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/networkInterfaces/{name}` | Create or replace |
| `PATCH` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/networkInterfaces/{name}` | Update |
| `GET` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/networkInterfaces/{name}` | Get |
| `DELETE` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/networkInterfaces/{name}` | Delete |
| `GET` | `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/networkInterfaces` | List in resource group |
| `GET` | `/subscriptions/{sub}/providers/Microsoft.Network/networkInterfaces` | List in subscription |


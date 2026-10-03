# terraform-provider-apipbx

Terraform provider for [apipbx](../apipbx), the API-only multi-tenant PBX.

    provider "apipbx" {
      endpoint = "https://pbx.example.com"   # or APIPBX_ENDPOINT
      token    = "..."                        # or APIPBX_TOKEN
    }

A platform API key can create tenants and manage every tenant; a tenant key only its own
tenant. A complete example is in [`examples/complete`](examples/complete/main.tf).

## Resources

| Resource | Import id | Notes |
|---|---|---|
| `apipbx_tenant` | `<tenant id>` | Deleting it deletes everything in it. Platform key only. |
| `apipbx_api_key` | `<key id>` | The secret (`key`, sensitive) exists only in the state of the apply that created it. |
| `apipbx_extension` | `<tenant id>/<id>` | `number` and `tech` force a new extension. `secret` is generated when omitted. |
| `apipbx_trunk` | `<tenant id>/<id>` | `name` and `tech` force a new trunk. A `port` cannot be cleared once set. |
| `apipbx_voicemail_box` | `<tenant id>/<id>` | Deleting it deletes its messages. `pin` is generated when omitted (leave it out if the owner sets it from the phone). Link it with an extension's `voicemail_box_id`, or use it as a `voicemail` destination. |
| `apipbx_ring_group` | `<tenant id>/<id>` | `members` are extension ids in ring order; `failover_type`/`failover_id` pick where an unanswered call goes (extension, ring_group, voicemail or hangup). |
| `apipbx_inbound_route` | `<tenant id>/<did>` | `destination_type`/`destination_id` (extension, ring_group, voicemail or hangup). |
| `apipbx_outbound_route` | `<tenant id>/<id>` | `patterns` (prefix, match, prepend) and `trunks` in failover order. |

Ids are strings so resources can reference each other (`members = [apipbx_extension.a.id]`).

## Replacing something that is still referenced

The API refuses to delete an extension, trunk or ring group while a ring group, route or
failover still refers to it (422). Changing a forces-new attribute such as an extension's
`number` therefore needs the new object to exist first:

    resource "apipbx_extension" "reception" {
      # ...
      lifecycle { create_before_destroy = true }
    }

## Development

    go test ./...
    go build -o terraform-provider-apipbx .

For a manual run against a local `apipbxd`, point Terraform's `dev_overrides` for
`registry.terraform.io/abtme/apipbx` at the directory holding the built binary and set
`APIPBX_ENDPOINT` / `APIPBX_TOKEN`. The full create / replan / update / import / destroy cycle
was run this way against the example before each release.

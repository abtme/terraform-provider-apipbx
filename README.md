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
| `apipbx_extension` | `<tenant id>/<id>` | `number` and `tech` force a new extension. `secret` is generated when omitted. `dnd` and the four `forward_*` numbers (always, busy, no answer, unreachable) are also changed by the phone's `*78`/`*79`/`*72`/`*73`, which shows as drift. |
| `apipbx_trunk` | `<tenant id>/<id>` | `name` and `tech` force a new trunk. A `port` cannot be cleared once set. |
| `apipbx_voicemail_box` | `<tenant id>/<id>` | Deleting it deletes its messages. `pin` is generated when omitted (leave it out if the owner sets it from the phone). Link it with an extension's `voicemail_box_id`, or use it as a `voicemail` destination. |
| `apipbx_time_group` | `<tenant id>/<id>` | `ranges` of `start`/`end` (`HH:MM`, end exclusive, `24:00` = end of day, an end before the start runs over midnight), optional `weekdays`, `month_days`, `months`, in an IANA `timezone`. |
| `apipbx_time_condition` | `<tenant id>/<id>` | `match_type`/`match_id` while the group matches, `no_match_type`/`no_match_id` otherwise; `override` (`none`, `match`, `no_match`) forces a branch; `toggle_code` lets `*28<code>` flip it from a phone (which shows as drift). |
| `apipbx_ivr` | `<tenant id>/<id>` | `entries` (`digit` 0-9 or `*`, `destination_type`/`destination_id`) in the order given; `announcement` is an Asterisk sound name (empty plays nothing) or `recording_id` an `apipbx_recording` (not both); `timeout_type`/`timeout_id` and `invalid_type`/`invalid_id` say where silence and unknown keys end up after `max_retries`. |
| `apipbx_recording` | not importable | A stored announcement: `content_base64 = filebase64("welcome.wav")`, 16-bit PCM mono 8000 Hz WAV (the API refuses anything else and says how to convert). Replacing the file updates it in place; audio changed outside Terraform shows up as drift. Deleting it is refused while an IVR plays it. |
| `apipbx_voicemail_greeting` | `<tenant id>/<box id>/<type>` | `type` is `unavailable` or `busy` (replace the default prompts), `name` or `temporary`; `content_base64 = filebase64("away.wav")`. A greeting recorded from the phone or replaced through the API shows as drift. After an import the configured file is applied on the next apply. |
| `apipbx_blacklist_entry` | `<tenant id>/<id>` | A caller number (digits, a leading + is dropped) whose calls from trunks are diverted. |
| `apipbx_blacklist_settings` | `<tenant id>` | One per tenant: `block_unknown` also diverts withheld caller ids; `destination_type`/`destination_id` say where diverted calls go (hangup by default). Destroying it restores the defaults. |
| `apipbx_queue` | `<tenant id>/<id>` | Queues: `strategy`, `agent_timeout`, `retry`, `wrapup_time`, `max_wait`, `max_callers`, `join_empty`, `leave_when_empty`, announcements, `music_on_hold`, `members` (`extension_id`, `penalty`, `dynamic`: in the queue only while logged in with `*45`) and `timeout_type`/`timeout_id`. Use PJSIP extensions as agents. Dialable by `number` and a destination (`queue`). |
| `apipbx_conference` | `<tenant id>/<id>` | A room dialled by `number`: `pin` (callers must enter it; empty = open), `admin_pin` (admins can lock the room and kick the last caller), `max_members` (0 = no limit), `mute_on_join`. A destination (`conference`). |
| `apipbx_announcement` | `<tenant id>/<id>` | Plays an `apipbx_recording`, then continues to `next_type`/`next_id`. A destination (`announcement`). |
| `apipbx_misc_destination` | `<tenant id>/<id>` | A `number` (internal, or external through the outbound routes) usable as a destination (`misc_destination`). |
| `apipbx_caller_id_step` | `<tenant id>/<id>` | Changes the caller id (`name_prefix` and/or `number`), then continues to `next_type`/`next_id`. A destination (`set_caller_id`). |
| `apipbx_speed_dial` | `<tenant id>/<id>` | `code` (1-4 digits) and `number`: extensions dial `*0` and the code. With `extension_id` it is that extension's own and wins over the tenant's of the same code. |
| `apipbx_paging_group` | `<tenant id>/<id>` | Dialling `number` pages the idle `members` (extension ids) not on DND; their phones are asked to answer by themselves. `duplex` lets them talk back. A destination (`paging_group`). |
| `apipbx_follow_me` | `<tenant id>/<extension id>` | An extension's follow-me: `numbers` (1-5, internal or external) ring together with the phone after `prering_seconds`; `confirm` makes an answerer press 1. |
| `apipbx_moh_class` | `<tenant id>/<id>` | A music on hold class: `recordings` (ids) played in order, or shuffled with `random`. A queue plays it by naming it in `music_on_hold`; `name` cannot change. |
| `apipbx_call_recording_policy` | `<tenant id>/<extension id>` | Which of an extension's calls are recorded: `inbound`, `outbound` or `both`. Destroying it turns recording off. The recordings themselves are read through the API. |
| `apipbx_parking_lot` | `<tenant id>` | A tenant's call parking: dial `park_number` to park a call, dial a slot (`first_slot`, `slots` of them) to pick it up. These numbers are reserved. |
| `apipbx_voicemail_group` | `<tenant id>/<id>` | A message left for `number` (or sent there by a route) goes to every mailbox in `boxes`. A destination (`voicemail_group`). |
| `apipbx_call_limit` | `<tenant id>` | The most calls a tenant may have in progress at once; one over it gets a busy signal. Destroying it removes the limit. |
| `apipbx_pin_set` | `<tenant id>/<id>` | A set of `pins` (a set of 3-12 digit PINs, sensitive); an outbound route with `pin_set_id` asks the caller for one of them before the call is placed. |
| `apipbx_disa` | `<tenant id>/<id>` | Direct inward system access: a destination (`disa`) where the caller keys in a PIN of `pin_set_id` and dials a number, which is called as an extension's call would be, with `caller_id` if set. |
| `apipbx_ring_group` | `<tenant id>/<id>` | `members` are extension ids in ring order; `failover_type`/`failover_id` pick where an unanswered call goes (extension, ring_group, voicemail, time_condition, ivr, queue, conference, announcement, misc_destination, set_caller_id or hangup). |
| `apipbx_inbound_route` | `<tenant id>/<did>` | `destination_type`/`destination_id` (extension, ring_group, voicemail, time_condition, ivr, queue, conference, announcement, misc_destination, set_caller_id or hangup). |
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

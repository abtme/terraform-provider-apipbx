terraform {
  required_providers { apipbx = { source = "abtme/apipbx" } }
}
provider "apipbx" {
  # endpoint = "https://pbx.example.com"  # or APIPBX_ENDPOINT
  # token    = "..."                      # or APIPBX_TOKEN (a platform key creates tenants)
}

resource "apipbx_tenant" "t" {
  slug = "tftest"
  name = "TF Test"
}

resource "apipbx_extension" "a" {
  tenant_id = apipbx_tenant.t.id
  number    = "1001"
  name      = "Reception"
  lifecycle { create_before_destroy = true }
}

resource "apipbx_extension" "b" {
  tenant_id    = apipbx_tenant.t.id
  number       = "1002"
  name         = "Sales"
  tech         = "iax2"
  outbound_cid = "+441234567890"
  ring_time    = 30
}

resource "apipbx_ring_group" "g" {
  tenant_id     = apipbx_tenant.t.id
  number        = "2000"
  name          = "Office"
  strategy      = "hunt"
  members       = [apipbx_extension.a.id, apipbx_extension.b.id]
  failover_type = "hangup"
}

resource "apipbx_trunk" "sip" {
  tenant_id = apipbx_tenant.t.id
  name      = "carrier"
  tech      = "pjsip"
  host      = "sip.example.net"
}

resource "apipbx_trunk" "iax" {
  tenant_id = apipbx_tenant.t.id
  name      = "magrathea"
  tech      = "iax2"
  host      = "iax.example.net"
  username  = "acct"
  secret    = "iaxsecret1"
  port      = 4569
}

resource "apipbx_inbound_route" "main" {
  tenant_id        = apipbx_tenant.t.id
  did              = "441234567891"
  description      = "main number"
  destination_type = "ring_group"
  destination_id   = apipbx_ring_group.g.id
}

resource "apipbx_outbound_route" "uk" {
  tenant_id = apipbx_tenant.t.id
  name      = "uk"
  position  = 10
  patterns = [
    { prefix = "0", match = "XXXXXXXXXX", prepend = "44" },
    { match = "NXXXXXX" },
  ]
  trunks = [apipbx_trunk.iax.id, apipbx_trunk.sip.id]
}

resource "apipbx_api_key" "k" {
  name      = "tenant-key"
  tenant_id = apipbx_tenant.t.id
}

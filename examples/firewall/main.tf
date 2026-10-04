# The server's firewall: limit the voice services to your own phones and your carriers, and let the carriers in whatever the
# restriction says. Needs a platform key. Nothing is enforced until apipbx_firewall.main has enabled = true: read the ruleset first
# (GET /v1/firewall/ruleset) and see docs/firewall.md in apipbx for how to turn it on without risking a lockout.
terraform {
  required_providers { apipbx = { source = "abtme/apipbx" } }
}
provider "apipbx" {}

variable "office" {
  type        = set(string)
  description = "Where your phones and your own administration come from."
}

# Built-in provider template: Magrathea's gateways and every address block it announces.
resource "apipbx_firewall_carrier" "magrathea" {
  provider_id = "magrathea"
}

# Any other provider: its own addresses and host names (resolved every few minutes).
resource "apipbx_firewall_carrier" "other" {
  provider_id = "custom"
  name        = "Other carrier"
  sources     = ["192.0.2.0/24"]
  hosts       = ["gw.other.example"]
}

# Phones register from the office only; carriers and trunks still get in.
resource "apipbx_firewall_restriction" "sip" {
  service = "sip"
  sources = var.office
}

resource "apipbx_firewall_restriction" "iax2" {
  service = "iax2"
  sources = var.office
}

resource "apipbx_firewall_restriction" "rtp" {
  service = "rtp"
  sources = var.office
}

# SSH and the API only from the office. The sources must include the address Terraform runs from, or this is refused.
resource "apipbx_firewall_restriction" "ssh" {
  service = "ssh"
  sources = var.office
}

resource "apipbx_firewall_rule" "scanner" {
  action   = "drop"
  protocol = "udp"
  port     = "5060"
  sources  = ["192.0.2.99"]
  comment  = "a scanner"
}

resource "apipbx_firewall" "main" {
  enabled    = false # true to enforce it
  depends_on = [apipbx_firewall_carrier.magrathea, apipbx_firewall_restriction.sip, apipbx_firewall_restriction.ssh]
}

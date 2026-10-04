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

resource "apipbx_voicemail_box" "reception" {
  tenant_id = apipbx_tenant.t.id
  number    = "1001"
  name      = "Reception"
  email     = "reception@example.com"
  attach    = true
}

resource "apipbx_voicemail_greeting" "reception_away" {
  tenant_id        = apipbx_tenant.t.id
  voicemail_box_id = apipbx_voicemail_box.reception.id
  type             = "unavailable"
  content_base64   = filebase64("away.wav") # 16-bit PCM, mono, 8000 Hz
}

resource "apipbx_extension" "a" {
  tenant_id        = apipbx_tenant.t.id
  number           = "1001"
  name             = "Reception"
  voicemail_box_id = apipbx_voicemail_box.reception.id
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
  failover_type = "voicemail"
  failover_id   = apipbx_voicemail_box.reception.id
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

resource "apipbx_time_group" "office" {
  tenant_id = apipbx_tenant.t.id
  name      = "Office hours"
  timezone  = "Europe/London"
  ranges = [
    { start = "09:00", end = "17:30", weekdays = ["mon", "tue", "wed", "thu", "fri"] },
  ]
}

# Office hours ring the ring group, any other time goes to voicemail.
resource "apipbx_time_condition" "office" {
  tenant_id     = apipbx_tenant.t.id
  name          = "Office"
  time_group_id = apipbx_time_group.office.id
  match_type    = "ring_group"
  match_id      = apipbx_ring_group.g.id
  no_match_type = "voicemail"
  no_match_id   = apipbx_voicemail_box.reception.id
}

resource "apipbx_inbound_route" "office_hours" {
  tenant_id        = apipbx_tenant.t.id
  did              = "441234567899"
  destination_type = "time_condition"
  destination_id   = apipbx_time_condition.office.id
}

# The greeting: 16-bit PCM, mono, 8000 Hz WAV
# (ffmpeg -i in.mp3 -ar 8000 -ac 1 -c:a pcm_s16le welcome.wav).
resource "apipbx_recording" "welcome" {
  tenant_id      = apipbx_tenant.t.id
  name           = "Welcome"
  content_base64 = filebase64("welcome.wav")
}

# A menu: 1 rings reception, 2 the ring group, or the caller dials an extension number directly;
# silence or an unknown key ends in voicemail.
resource "apipbx_ivr" "main" {
  tenant_id       = apipbx_tenant.t.id
  name            = "Main menu"
  recording_id    = apipbx_recording.welcome.id # or announcement = "custom/welcome" for a sound file on the Asterisk host
  timeout_seconds = 8
  direct_dial     = ["extension"]
  entries = [
    { digit = "1", destination_type = "extension", destination_id = apipbx_extension.a.id },
    { digit = "2", destination_type = "ring_group", destination_id = apipbx_ring_group.g.id },
  ]
  timeout_type = "voicemail"
  timeout_id   = apipbx_voicemail_box.reception.id
  invalid_type = "voicemail"
  invalid_id   = apipbx_voicemail_box.reception.id
}

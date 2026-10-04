# How the server backs itself up: every day at 03:30, the newest 21 kept on the server, and a copy of each on a NAS over SFTP.
resource "apipbx_backup_settings" "this" {
  time        = "03:30"
  keep        = 21
  recordings  = true

  remote_enabled = true
  remote_host    = "nas.example.net"
  remote_user    = "backup"
  remote_path    = "/volume1/apipbx"
  remote_keep    = 30
}

# Add this key to the remote user's ~/.ssh/authorized_keys (the server makes it when the remote is enabled).
output "backup_public_key" {
  value = apipbx_backup_settings.this.public_key
}

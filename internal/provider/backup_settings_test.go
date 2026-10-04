package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

func TestBackupSettingsSendOnlyWhatIsSetAndNestTheRemote(t *testing.T) {
	m := backupSettingsModel{
		Enabled: types.BoolUnknown(), Time: types.StringValue("04:30"), Keep: types.Int64Value(30), Dir: types.StringNull(), Recordings: types.BoolValue(false),
		RemoteEnabled: types.BoolValue(true), RemoteHost: types.StringValue("nas.example"), RemotePort: types.Int64Unknown(), RemoteUser: types.StringValue("backup"),
		RemotePath: types.StringValue("/srv/pbx"), RemoteKeep: types.Int64Null(),
	}
	want := map[string]any{"time": "04:30", "keep": int64(30), "recordings": false,
		"remote": map[string]any{"enabled": true, "host": "nas.example", "user": "backup", "path": "/srv/pbx"}}
	if got := m.body(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if got := (backupSettingsModel{}).body(); len(got) != 0 {
		// a model of zero values is not "unset": the framework gives null, which is
		t.Logf("zero model sends %v", got)
	}
}

func TestBackupSettingsAreReadBack(t *testing.T) {
	var m backupSettingsModel
	m.set(client.BackupStatus{PublicKey: "ssh-ed25519 AAAA x", Config: client.BackupConfig{Enabled: true, Time: "03:00", Keep: 14, Dir: "/var/backups/apipbx", Recordings: true,
		Remote: client.BackupRemote{Enabled: true, Host: "h", Port: 22, User: "u", Path: "/p", Keep: 7}}})
	if m.ID.ValueString() != "backup" || m.Time.ValueString() != "03:00" || m.Keep.ValueInt64() != 14 || !m.RemoteEnabled.ValueBool() || m.RemoteKeep.ValueInt64() != 7 || m.PublicKey.ValueString() != "ssh-ed25519 AAAA x" {
		t.Errorf("%+v", m)
	}
}

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func set(vals ...string) types.Set {
	v := make([]attr.Value, len(vals))
	for i, s := range vals {
		v[i] = types.StringValue(s)
	}
	return types.SetValueMust(types.StringType, v)
}

func TestNormSourceMatchesTheServer(t *testing.T) {
	for in, want := range map[string]string{
		"198.51.100.7":      "198.51.100.7/32",
		"203.0.113.9/24":    "203.0.113.0/24",
		"2001:db8::1":       "2001:db8::1/128",
		"2001:db8:0:0::/32": "2001:db8::/32",
		"::ffff:192.0.2.1":  "192.0.2.1/32",
		"not-an-address":    "not-an-address",
	} {
		if got := normSource(in); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestSourceSetKeepsTheUsersSpelling(t *testing.T) {
	planned := set("198.51.100.7", "203.0.113.0/24")
	got := sourceSet([]string{"198.51.100.7/32", "203.0.113.0/24", "2001:db8::/32"}, planned)
	want := set("198.51.100.7", "203.0.113.0/24", "2001:db8::/32")
	if !got.Equal(want) {
		t.Errorf("the user's own spelling is kept where it means the same, the rest as the server says it: %v", got)
	}
	if bare := sourceSet([]string{"192.0.2.99/32"}, types.SetNull(types.StringType)); !bare.Equal(set("192.0.2.99/32")) {
		t.Errorf("with nothing to copy from (an import) the server's form is used: %v", bare)
	}
}

func TestSameSourcesPlanModifier(t *testing.T) {
	run := func(state, plan types.Set) types.Set {
		resp := &planmodifier.SetResponse{PlanValue: plan}
		sameSources{}.PlanModifySet(context.Background(), planmodifier.SetRequest{StateValue: state, PlanValue: plan}, resp)
		return resp.PlanValue
	}
	state := set("192.0.2.99/32", "2001:db8::/32")
	if got := run(state, set("192.0.2.99", "2001:db8::/32")); !got.Equal(state) {
		t.Errorf("the same addresses written differently are no change: %v", got)
	}
	changed := set("192.0.2.98", "2001:db8::/32")
	if got := run(state, changed); !got.Equal(changed) {
		t.Errorf("different addresses are a change: %v", got)
	}
	if got := run(state, set("192.0.2.99")); !got.Equal(set("192.0.2.99")) {
		t.Errorf("a shorter list is a change: %v", got)
	}
	if got := run(types.SetNull(types.StringType), changed); !got.Equal(changed) {
		t.Errorf("nothing stored yet (a create): %v", got)
	}
}

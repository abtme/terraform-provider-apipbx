package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-apipbx/internal/client"
)

func idString(id int64) types.String { return types.StringValue(strconv.FormatInt(id, 10)) }

func parseID(d *diag.Diagnostics, what string, s types.String) int64 {
	n, err := strconv.ParseInt(s.ValueString(), 10, 64)
	if err != nil {
		d.AddError("invalid "+what, fmt.Sprintf("%q is not a numeric id", s.ValueString()))
	}
	return n
}

func isNotFound(err error) bool { return errors.Is(err, client.ErrNotFound) }

// clientFrom extracts the API client handed over by the provider's Configure.
func clientFrom(data any, c **client.Client) {
	if v, ok := data.(*client.Client); ok {
		*c = v
	}
}

// readFailed removes the resource from state when it is gone, and reports any other error.
func readFailed(ctx context.Context, what string, err error, resp *resource.ReadResponse) {
	if isNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.AddError("read "+what, err.Error())
}

// deleteFailed reports a delete error; an object that is already gone is not one.
func deleteFailed(what string, err error, resp *resource.DeleteResponse) {
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete "+what, err.Error())
	}
}

// importTenantScoped accepts "<tenant id>/<object id>" and sets tenant_id and key.
func importTenantScoped(ctx context.Context, key string, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tenant, id, ok := strings.Cut(req.ID, "/")
	if !ok || tenant == "" || id == "" {
		resp.Diagnostics.AddError("invalid import id", `expected "<tenant id>/<id>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_id"), tenant)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(key), id)...)
}

// idList converts a list of string ids to numbers.
func idList(d *diag.Diagnostics, what string, l types.List) []int64 {
	out := []int64{}
	for _, e := range l.Elements() {
		s, ok := e.(types.String)
		if !ok {
			d.AddError("invalid "+what, "expected a list of ids")
			return nil
		}
		out = append(out, parseID(d, what, s))
	}
	return out
}

func idListValue(ids []int64) types.List {
	elems := make([]attr.Value, 0, len(ids))
	for _, id := range ids {
		elems = append(elems, idString(id))
	}
	return types.ListValueMust(types.StringType, elems)
}

func ptr[T any](v T) *T { return &v }

// toDestination builds an API destination from the (type, id) attribute pair.
func toDestination(d *diag.Diagnostics, typ, id types.String) *client.Destination {
	dest := &client.Destination{Type: typ.ValueString()}
	if !id.IsNull() && !id.IsUnknown() {
		dest.ID = ptr(parseID(d, "destination id", id))
	}
	return dest
}

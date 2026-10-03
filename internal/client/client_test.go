package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotFoundIsDetectable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k").GetTenant(context.Background(), 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestInboundRouteDIDIsEscapedAndAuthSent(t *testing.T) {
	var path, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.EscapedPath(), r.Header.Get("Authorization")
		w.Write([]byte(`{"did":"+441234567891","tenant_id":7}`))
	}))
	defer srv.Close()
	rt, err := New(srv.URL+"/", "secret").GetInboundRoute(context.Background(), 7, "+441234567891")
	if err != nil || rt.DID != "+441234567891" {
		t.Fatalf("%+v %v", rt, err)
	}
	if path != "/v1/tenants/7/inbound-routes/+441234567891" && path != "/v1/tenants/7/inbound-routes/%2B441234567891" {
		t.Fatalf("path %q", path)
	}
	if auth != "Bearer secret" {
		t.Fatalf("auth %q", auth)
	}
}

func TestAPIKeyLookupScansTheList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"id":1,"name":"a"},{"id":2,"name":"b","tenant_id":5}]`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	k, err := c.GetAPIKey(context.Background(), 2)
	if err != nil || k.Name != "b" || k.TenantID == nil || *k.TenantID != 5 {
		t.Fatalf("%+v %v", k, err)
	}
	if _, err := c.GetAPIKey(context.Background(), 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key: %v", err)
	}
}

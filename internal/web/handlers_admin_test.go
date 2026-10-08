package web

import (
	"net/http"
	"testing"

	"iperf3-tracker/internal/model"
)

func TestAdminRoutes(t *testing.T) {
	srv := newTestServer(t)
	admin := login(t, srv, "admin", "admin123")
	call(t, srv, "POST", "/api/auth/register", admin, `{"username":"bob","email":"bob@example.org","password":"geheim1"}`, nil)
	user := login(t, srv, "bob", "geheim1")

	for _, path := range []string{"/api/admin/cleanup/tests?all=true", "/api/admin/cleanup/traces?all=true"} {
		expectStatus(t, srv, "DELETE", path, "", "", http.StatusUnauthorized)
		expectStatus(t, srv, "DELETE", path, user, "", http.StatusForbidden)
	}
	expectStatus(t, srv, "GET", "/api/admin/stats/database", user, "", http.StatusForbidden)

	// Ohne Einschränkung wird nichts gelöscht.
	expectStatus(t, srv, "DELETE", "/api/admin/cleanup/tests", admin, "", http.StatusBadRequest)
	expectStatus(t, srv, "DELETE", "/api/admin/cleanup/traces", admin, "", http.StatusBadRequest)
	expectStatus(t, srv, "DELETE", "/api/admin/cleanup/tests?days=-1", admin, "", http.StatusUnprocessableEntity)

	var sv model.Server
	call(t, srv, "POST", "/api/servers", admin, `{"name":"A","host":"a"}`, &sv)
	var test model.Test
	call(t, srv, "POST", "/api/tests/run", admin, `{"server_id":`+itoa(sv.ID)+`}`, &test)
	waitTestDone(t, srv, admin, test.ID)

	var stats map[string]any
	if code := call(t, srv, "GET", "/api/admin/stats/database", admin, "", &stats); code != http.StatusOK ||
		stats["total_tests"] != 1.0 || stats["total_users"] != 2.0 || stats["oldest_test"] == nil {
		t.Errorf("stats: %d, %v", code, stats)
	}

	var res map[string]any
	if call(t, srv, "DELETE", "/api/admin/cleanup/tests?days=1", admin, "", &res); res["deleted_count"] != 0.0 {
		t.Errorf("days=1 darf den neuen Test nicht löschen: %v", res)
	}
	if call(t, srv, "DELETE", "/api/admin/cleanup/tests?server_id="+itoa(sv.ID), admin, "", &res); res["deleted_count"] != 1.0 {
		t.Errorf("server_id: %v", res)
	}
	if code := call(t, srv, "DELETE", "/api/admin/cleanup/traces?all=true", admin, "", &res); code != http.StatusOK ||
		res["deleted_traces"] != 0.0 || res["deleted_hops"] != 0.0 {
		t.Errorf("traces all: %d, %v", code, res)
	}
}

func TestPublicServers(t *testing.T) {
	srv := newTestServer(t)

	var all []model.PublicServer
	if code := call(t, srv, "GET", "/api/public-servers", "", "", &all); code != http.StatusOK || len(all) != 10 {
		t.Fatalf("Liste ohne Anmeldung: %d, %d Einträge", code, len(all))
	}
	var found []model.PublicServer
	if call(t, srv, "GET", "/api/public-servers/search?query=DEUTSCHLAND", "", "", &found); len(found) != 2 {
		t.Errorf("Suche nach Ort: %+v", found)
	}
	if call(t, srv, "GET", "/api/public-servers/search?query=init7", "", "", &found); len(found) != 1 || found[0].Host != "speedtest.init7.net" {
		t.Errorf("Suche nach Anbieter: %+v", found)
	}
	expectStatus(t, srv, "GET", "/api/public-servers/search", "", "", http.StatusUnprocessableEntity)
}

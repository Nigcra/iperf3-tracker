package web

import (
	"net/http"
	"testing"
	"time"

	"iperf3-tracker/internal/model"
)

func TestTraceRoutes(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	expectStatus(t, srv, "GET", "/api/traces", "", "", http.StatusUnauthorized)
	expectStatus(t, srv, "POST", "/api/traces", tok, `{"destination":""}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/traces", tok, `{"destination":"x","max_hops":65}`, http.StatusUnprocessableEntity)

	// Direkter Trace: Ohne traceroute-Binary wird er als unvollständig gespeichert.
	var direct model.Trace
	if code := call(t, srv, "POST", "/api/traces", tok, `{"destination":"127.0.0.1"}`, &direct); code != http.StatusCreated {
		t.Fatalf("POST /traces: %d", code)
	}
	if direct.ID == 0 || direct.Completed || direct.ErrorMessage == nil || direct.TestID != nil || direct.Hops == nil {
		t.Errorf("direkter Trace: %+v", direct)
	}

	var got model.Trace
	if code := call(t, srv, "GET", "/api/traces/"+itoa(direct.ID), tok, "", &got); code != http.StatusOK || got.ID != direct.ID {
		t.Errorf("GET /traces/{id}: %d", code)
	}
	var list []model.Trace
	if call(t, srv, "GET", "/api/traces?limit=5", tok, "", &list); len(list) != 1 {
		t.Errorf("Liste: %d", len(list))
	}
	if call(t, srv, "GET", "/api/traces/recent", tok, "", &list); len(list) != 0 {
		t.Errorf("recent darf unvollständige Traces nicht enthalten: %d", len(list))
	}

	// Trace zu einem Test: läuft im Hintergrund.
	var sv model.Server
	call(t, srv, "POST", "/api/servers", tok, `{"name":"A","host":"127.0.0.1"}`, &sv)
	var test model.Test
	call(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+itoa(sv.ID)+`}`, &test)
	path := "/api/tests/" + itoa(test.ID) + "/trace"

	expectStatus(t, srv, "GET", path, tok, "", http.StatusNotFound)
	expectStatus(t, srv, "POST", path, tok, "", http.StatusAccepted)
	deadline := time.Now().Add(5 * time.Second)
	var tt model.Trace
	for call(t, srv, "GET", path, tok, "", &tt) != http.StatusOK {
		if time.Now().After(deadline) {
			t.Fatal("Trace zum Test nicht rechtzeitig gespeichert")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if tt.TestID == nil || *tt.TestID != test.ID || tt.DestinationHost != "127.0.0.1" {
		t.Errorf("Trace zum Test: %+v", tt)
	}
	expectStatus(t, srv, "POST", path, tok, "", http.StatusBadRequest)
	if call(t, srv, "GET", "/api/traces/test/"+itoa(test.ID), tok, "", &list); len(list) != 1 {
		t.Errorf("/traces/test/{id}: %d", len(list))
	}
	expectStatus(t, srv, "POST", "/api/tests/999/trace", tok, "", http.StatusNotFound)

	// Löschen.
	expectStatus(t, srv, "DELETE", path, tok, "", http.StatusOK)
	expectStatus(t, srv, "DELETE", path, tok, "", http.StatusNotFound)
	expectStatus(t, srv, "DELETE", "/api/traces/"+itoa(direct.ID), tok, "", http.StatusOK)
	expectStatus(t, srv, "GET", "/api/traces/"+itoa(direct.ID), tok, "", http.StatusNotFound)
}

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"iperf3-tracker/internal/model"
)

// waitTestDone pollt den Live-Status, bis der Test nicht mehr wartet oder läuft.
func waitTestDone(t *testing.T, srv *httptest.Server, tok string, id int64) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var live map[string]any
		if code := call(t, srv, "GET", "/api/tests/"+itoa(id)+"/live", tok, "", &live); code != http.StatusOK {
			t.Fatalf("live: Status %d", code)
		}
		if s := live["status"]; s == "completed" || s == "failed" {
			return live
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Test %d nicht rechtzeitig beendet", id)
	return nil
}

func TestRunAndQueryTests(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	var sv model.Server
	if code := call(t, srv, "POST", "/api/servers", tok, `{"name":"A","host":"127.0.0.1"}`, &sv); code != http.StatusCreated {
		t.Fatalf("Server anlegen: %d", code)
	}
	var off model.Server
	call(t, srv, "POST", "/api/servers", tok, `{"name":"Aus","host":"x","enabled":false}`, &off)

	expectStatus(t, srv, "POST", "/api/tests/run", "", `{"server_id":1}`, http.StatusUnauthorized)
	expectStatus(t, srv, "POST", "/api/tests/run", tok, `{"server_id":999}`, http.StatusBadRequest)
	expectStatus(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+itoa(off.ID)+`}`, http.StatusBadRequest)
	expectStatus(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+itoa(sv.ID)+`,"duration":0}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+itoa(sv.ID)+`,"direction":"seitwärts"}`, http.StatusUnprocessableEntity)

	// Start: sofort 201 mit Status pending und Vorgabewerten.
	var test model.Test
	if code := call(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+itoa(sv.ID)+`,"direction":"bidirectional"}`, &test); code != http.StatusCreated {
		t.Fatalf("Test starten: %d", code)
	}
	if test.Status != model.StatusPending || test.Duration != 10 || test.ParallelStreams != 1 || test.Protocol != model.ProtocolTCP {
		t.Errorf("neuer Test: %+v", test)
	}

	// Der Test-Runner hat keine iperf3-Binary: der Test schlägt fehl, der
	// Live-Status bleibt aber noch als „laufend“ abrufbar.
	live := waitTestDone(t, srv, tok, test.ID)
	if live["status"] != "failed" || live["is_running"] != true || live["server_name"] != "A" {
		t.Errorf("live: %v", live)
	}

	var detail map[string]any
	if code := call(t, srv, "GET", "/api/tests/"+itoa(test.ID), tok, "", &detail); code != http.StatusOK {
		t.Fatalf("Detail: %d", code)
	}
	if detail["status"] != "failed" || detail["error_message"] == nil || detail["server"].(map[string]any)["name"] != "A" {
		t.Errorf("Detail: %v", detail)
	}
	if _, ok := detail["raw_output"]; !ok {
		t.Error("Detail ohne raw_output")
	}

	var list []map[string]any
	call(t, srv, "GET", "/api/tests", tok, "", &list)
	if len(list) != 1 {
		t.Fatalf("Liste: %d Einträge", len(list))
	}
	if _, ok := list[0]["raw_output"]; ok {
		t.Error("Liste darf raw_output nicht enthalten")
	}
	for query, want := range map[string]int{
		"status=failed":             1,
		"status=completed":          0,
		"server_id=" + itoa(sv.ID):  1,
		"server_id=" + itoa(off.ID): 0,
		"from_date=" + url.QueryEscape(time.Now().Add(-time.Hour).Format(time.RFC3339)): 1,
		"to_date=2000-01-01": 0,
	} {
		call(t, srv, "GET", "/api/tests?"+query, tok, "", &list)
		if len(list) != want {
			t.Errorf("Filter %s: %d Einträge, erwartet %d", query, len(list), want)
		}
	}
	expectStatus(t, srv, "GET", "/api/tests?status=kaputt", tok, "", http.StatusUnprocessableEntity)
	expectStatus(t, srv, "GET", "/api/tests?from_date=gestern", tok, "", http.StatusUnprocessableEntity)

	// Kein erfolgreicher Test: latest liefert null.
	var latest any = "unverändert"
	if code := call(t, srv, "GET", "/api/tests/server/"+itoa(sv.ID)+"/latest", tok, "", &latest); code != http.StatusOK || latest != nil {
		t.Errorf("latest: %d, %v", code, latest)
	}

	expectStatus(t, srv, "GET", "/api/tests/999", tok, "", http.StatusNotFound)
	expectStatus(t, srv, "GET", "/api/tests/999/live", tok, "", http.StatusNotFound)
	expectStatus(t, srv, "DELETE", "/api/tests/"+itoa(test.ID), tok, "", http.StatusNoContent)
	expectStatus(t, srv, "DELETE", "/api/tests/"+itoa(test.ID), tok, "", http.StatusNotFound)

	// Löschen eines Servers entfernt seine Tests mit.
	call(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+itoa(sv.ID)+`}`, &test)
	waitTestDone(t, srv, tok, test.ID)
	expectStatus(t, srv, "DELETE", "/api/servers/"+itoa(sv.ID), tok, "", http.StatusNoContent)
	expectStatus(t, srv, "GET", "/api/tests/"+itoa(test.ID), tok, "", http.StatusNotFound)
}

func TestUDPBandwidth(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	var plain model.Server
	call(t, srv, "POST", "/api/servers", tok, `{"name":"ohne","host":"a"}`, &plain)
	if plain.DefaultUDPBandwidthMbps != nil {
		t.Errorf("Vorgabe ohne Angabe: %v", *plain.DefaultUDPBandwidthMbps)
	}
	expectStatus(t, srv, "POST", "/api/servers", tok, `{"name":"x","host":"x","default_udp_bandwidth_mbps":0}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/servers", tok, `{"name":"x","host":"x","default_udp_bandwidth_mbps":100001}`, http.StatusUnprocessableEntity)

	var sv model.Server
	if code := call(t, srv, "POST", "/api/servers", tok, `{"name":"udp","host":"b","default_udp_bandwidth_mbps":500}`, &sv); code != http.StatusCreated ||
		sv.DefaultUDPBandwidthMbps == nil || *sv.DefaultUDPBandwidthMbps != 500 {
		t.Fatalf("Server mit UDP-Vorgabe: %d, %+v", code, sv)
	}

	start := func(body string) model.Test {
		t.Helper()
		var test model.Test
		if code := call(t, srv, "POST", "/api/tests/run", tok, body, &test); code != http.StatusCreated {
			t.Fatalf("Test starten (%s): %d", body, code)
		}
		waitTestDone(t, srv, tok, test.ID)
		return test
	}
	id := itoa(sv.ID)
	if test := start(`{"server_id":` + id + `,"protocol":"udp"}`); test.UDPBandwidthMbps == nil || *test.UDPBandwidthMbps != 500 {
		t.Errorf("UDP ohne Angabe übernimmt Server-Vorgabe nicht: %v", test.UDPBandwidthMbps)
	}
	if test := start(`{"server_id":` + id + `,"protocol":"udp","udp_bandwidth_mbps":42.5}`); test.UDPBandwidthMbps == nil || *test.UDPBandwidthMbps != 42.5 {
		t.Errorf("UDP mit Angabe: %v", test.UDPBandwidthMbps)
	}
	if test := start(`{"server_id":` + id + `,"protocol":"tcp","udp_bandwidth_mbps":42.5}`); test.UDPBandwidthMbps != nil {
		t.Errorf("TCP darf keine UDP-Bandbreite speichern: %v", *test.UDPBandwidthMbps)
	}
	expectStatus(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+id+`,"protocol":"udp","udp_bandwidth_mbps":-1}`, http.StatusUnprocessableEntity)

	// Partielles Update kann die Vorgabe wieder entfernen.
	var upd model.Server
	call(t, srv, "PUT", "/api/servers/"+id, tok, `{"default_udp_bandwidth_mbps":null}`, &upd)
	if upd.DefaultUDPBandwidthMbps != nil {
		t.Errorf("Vorgabe nach null: %v", *upd.DefaultUDPBandwidthMbps)
	}
}

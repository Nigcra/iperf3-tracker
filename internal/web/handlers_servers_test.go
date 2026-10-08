package web

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"iperf3-tracker/internal/model"
)

func TestServerCRUD(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	expectStatus(t, srv, "GET", "/api/servers", "", "", http.StatusUnauthorized)

	// Anlegen mit Minimaldaten: Vorgabewerte wie im Python-Schema.
	var a model.Server
	if code := call(t, srv, "POST", "/api/servers", tok, `{"name":"A","host":"a.example.org","id":999}`, &a); code != http.StatusCreated {
		t.Fatalf("Anlegen: Status %d", code)
	}
	want := model.DefaultServerSettings()
	want.Name, want.Host = "A", "a.example.org"
	if a.ServerSettings != want {
		t.Errorf("Vorgabewerte:\n got %+v\nwant %+v", a.ServerSettings, want)
	}
	if a.ID == 999 || a.CreatedAt.IsZero() || !a.CreatedAt.Equal(a.UpdatedAt) {
		t.Errorf("ID/Zeitstempel unerwartet: %+v", a)
	}

	expectStatus(t, srv, "POST", "/api/servers", tok, `{"name":"A","host":"x"}`, http.StatusBadRequest)
	expectStatus(t, srv, "POST", "/api/servers", tok, `{"name":"B","host":"b","port":0}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/servers", tok, `{"name":"B","host":"b","default_protocol":"icmp"}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/servers", tok, `{"name":"","host":"b"}`, http.StatusUnprocessableEntity)

	var b model.Server
	if code := call(t, srv, "POST", "/api/servers", tok, `{"name":"B","host":"b.example.org","enabled":false,"description":"Test"}`, &b); code != http.StatusCreated {
		t.Fatalf("Anlegen B: Status %d", code)
	}

	// Liste: neueste zuerst, Filter enabled.
	var list []model.Server
	if call(t, srv, "GET", "/api/servers", tok, "", &list); len(list) != 2 || list[0].Name != "B" {
		t.Errorf("Liste: %+v", list)
	}
	if call(t, srv, "GET", "/api/servers?enabled=true", tok, "", &list); len(list) != 1 || list[0].Name != "A" {
		t.Errorf("Filter enabled=true: %+v", list)
	}
	if call(t, srv, "GET", "/api/servers?limit=1&skip=1", tok, "", &list); len(list) != 1 || list[0].Name != "A" {
		t.Errorf("Paging: %+v", list)
	}
	expectStatus(t, srv, "GET", "/api/servers?limit=0", tok, "", http.StatusUnprocessableEntity)
	expectStatus(t, srv, "GET", "/api/servers?enabled=vielleicht", tok, "", http.StatusUnprocessableEntity)

	// Partielles Update: nur übergebene Felder ändern sich.
	time.Sleep(2 * time.Millisecond)
	var upd model.Server
	if code := call(t, srv, "PUT", "/api/servers/"+itoa(b.ID), tok, `{"port":5202,"description":null,"created_at":"2000-01-01T00:00:00Z"}`, &upd); code != http.StatusOK {
		t.Fatalf("Update: Status %d", code)
	}
	if upd.Port != 5202 || upd.Description != nil || upd.Name != "B" || upd.Enabled ||
		!upd.CreatedAt.Equal(b.CreatedAt) || !upd.UpdatedAt.After(b.UpdatedAt) {
		t.Errorf("Update: %+v", upd)
	}
	var got model.Server
	if call(t, srv, "GET", "/api/servers/"+itoa(b.ID), tok, "", &got); got != upd {
		t.Errorf("gespeichert != Antwort:\n got %+v\nwant %+v", got, upd)
	}
	expectStatus(t, srv, "PUT", "/api/servers/"+itoa(b.ID), tok, `{"name":"A"}`, http.StatusBadRequest)
	expectStatus(t, srv, "PUT", "/api/servers/"+itoa(b.ID), tok, `{"default_duration":301}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "PUT", "/api/servers/999", tok, `{}`, http.StatusNotFound)

	// Löschen.
	expectStatus(t, srv, "DELETE", "/api/servers/"+itoa(b.ID), tok, "", http.StatusNoContent)
	expectStatus(t, srv, "DELETE", "/api/servers/"+itoa(b.ID), tok, "", http.StatusNotFound)
	expectStatus(t, srv, "GET", "/api/servers/"+itoa(b.ID), tok, "", http.StatusNotFound)
	expectStatus(t, srv, "GET", "/api/servers/abc", tok, "", http.StatusUnprocessableEntity)
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

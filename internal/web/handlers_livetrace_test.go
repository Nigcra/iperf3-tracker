package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/trace"
)

// TestLiveTraceHelper ersetzt tracert/traceroute in TestLiveTrace: drei Hops,
// der letzte ist das Ziel 127.0.0.1, danach eine Zeile, die nicht mehr
// gelesen werden darf.
func TestLiveTraceHelper(t *testing.T) {
	if os.Getenv("LIVE_TRACE_HELPER") != "1" {
		return
	}
	for _, line := range []string{
		"traceroute to 127.0.0.1 (127.0.0.1), 30 hops max",
		" 1  10.0.0.1  1.0 ms  1.0 ms  1.0 ms",
		" 2  * * *",
		" 3  127.0.0.1  2.0 ms  2.0 ms  2.0 ms",
		" 4  10.9.9.9  9.0 ms",
	} {
		fmt.Println(line)
		time.Sleep(20 * time.Millisecond)
	}
	os.Exit(0)
}

type sseEvent struct {
	Type        string           `json:"type"`
	Destination string           `json:"destination"`
	Message     string           `json:"message"`
	Data        model.TraceHop   `json:"data"`
	Hops        []model.TraceHop `json:"hops"`
	TraceID     int64            `json:"trace_id"`
}

// readSSE liest alle data-Events, bis der Server die Verbindung schließt.
func readSSE(t *testing.T, srv *httptest.Server, destination, token string) []sseEvent {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/live-trace/stream/" + url.PathEscape(destination) + "?token=" + url.QueryEscape(token))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var events []sseEvent
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue
		}
		var ev sseEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Fatalf("Event nicht lesbar: %s", data)
		}
		events = append(events, ev)
	}
	return events
}

func eventTypes(events []sseEvent) string {
	types := make([]string, len(events))
	for i, ev := range events {
		types[i] = ev.Type
	}
	return strings.Join(types, ",")
}

func TestLiveTrace(t *testing.T) {
	t.Setenv("LIVE_TRACE_HELPER", "1")
	srv := newTestServerWithTracer(t, trace.NewTracer(nil, os.Args[0], "-test.run=^TestLiveTraceHelper$", "--"))
	tok := login(t, srv, "admin", "admin123")

	events := readSSE(t, srv, "127.0.0.1", tok)
	if got := eventTypes(events); got != "start,hop,hop,hop,interpolation_complete,complete" {
		t.Fatalf("Events: %s", got)
	}
	if events[0].Destination != "127.0.0.1" {
		t.Errorf("start: %+v", events[0])
	}
	if h := events[3].Data; h.HopNumber != 3 || h.IPAddress == nil || *h.IPAddress != "127.0.0.1" {
		t.Errorf("letzter Hop: %+v", h)
	}
	if len(events[4].Hops) != 3 || events[4].Hops[1].Responded {
		t.Errorf("interpolation_complete: %+v", events[4].Hops)
	}

	// Gespeichert, bevor "complete" gesendet wird.
	var saved model.Trace
	if code := call(t, srv, "GET", "/api/traces/"+itoa(events[5].TraceID), tok, "", &saved); code != http.StatusOK {
		t.Fatalf("gespeicherter Trace: %d", code)
	}
	if !saved.Completed || saved.TestID != nil || len(saved.Hops) != 3 || saved.DestinationHost != "127.0.0.1" {
		t.Errorf("gespeicherter Trace: %+v", saved)
	}
}

func TestLiveTraceErrors(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	// Ungültiges Token: error-Event statt HTTP-Fehler (EventSource sieht keinen Status).
	if events := readSSE(t, srv, "127.0.0.1", "kaputt"); eventTypes(events) != "error" || events[0].Message == "" {
		t.Errorf("ohne gültiges Token: %+v", events)
	}
	// Ohne traceroute-Binary und ohne Hops: start, dann error; nichts gespeichert.
	events := readSSE(t, srv, "127.0.0.1", tok)
	if eventTypes(events) != "start,error" || !strings.Contains(events[1].Message, "nicht gefunden") {
		t.Errorf("ohne Werkzeug: %+v", events)
	}
	var list []model.Trace
	if call(t, srv, "GET", "/api/traces", tok, "", &list); len(list) != 0 {
		t.Errorf("fehlgeschlagener Live-Trace wurde gespeichert: %d", len(list))
	}
}

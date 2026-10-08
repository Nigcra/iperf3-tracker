package web

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestServesUI(t *testing.T) {
	srv := newTestServer(t)
	for path, want := range map[string]string{
		"/":              "<title>iperf3-Tracker</title>",
		"/static/app.js": "async function api(",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), want) {
			t.Errorf("%s: Status %d, enthält %q nicht", path, resp.StatusCode, want)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("%s: Cache-Control = %q", path, cc)
		}
	}
	expectStatus(t, srv, "GET", "/static/gibt-es-nicht.js", "", "", http.StatusNotFound)
}

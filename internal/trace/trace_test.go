package trace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"iperf3-tracker/internal/model"
)

func parseFixture(t *testing.T, name string) []rawHop {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var hops []rawHop
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if h, ok := parseHopLine(sc.Text()); ok {
			hops = append(hops, h)
		}
	}
	return hops
}

func rtt(h rawHop) float64 {
	if h.RTTMs == nil {
		return -1
	}
	return *h.RTTMs
}

func TestParseTracertWindows(t *testing.T) {
	// Echte Ausgabe von `tracert -d` auf deutschem Windows (Codepage 850).
	hops := parseFixture(t, "tracert_windows_de.txt")
	if len(hops) != 10 {
		t.Fatalf("%d Hops, erwartet 10", len(hops))
	}
	if hops[0].IP != "10.10.80.1" || rtt(hops[0]) != 1 {
		t.Errorf("Hop 1 (<1 ms): %+v rtt=%v", hops[0], rtt(hops[0]))
	}
	if hops[1].Number != 2 || hops[1].IP != "" || hops[1].RTTMs != nil {
		t.Errorf("Hop 2 (Timeout): %+v", hops[1])
	}
	if hops[4].IP != "188.111.129.216" || rtt(hops[4]) != 2.33 {
		t.Errorf("Hop 5: %+v rtt=%v", hops[4], rtt(hops[4]))
	}
	if last := hops[9]; last.Number != 10 || last.IP != "8.8.8.8" {
		t.Errorf("letzter Hop: %+v", last)
	}
}

func TestParseTracerouteLinux(t *testing.T) {
	hops := parseFixture(t, "traceroute_linux.txt")
	if len(hops) != 6 {
		t.Fatalf("%d Hops, erwartet 6", len(hops))
	}
	if hops[0].IP != "10.10.80.1" || rtt(hops[0]) != 0.39 {
		t.Errorf("Hop 1: %+v rtt=%v", hops[0], rtt(hops[0]))
	}
	if hops[1].IP != "" {
		t.Errorf("Hop 2 (* * *): %+v", hops[1])
	}
	// Teilweiser Timeout zählt als beantwortet.
	if hops[2].IP != "92.79.253.252" || rtt(hops[2]) != 2.21 {
		t.Errorf("Hop 3 (teilweiser Timeout): %+v rtt=%v", hops[2], rtt(hops[2]))
	}
	// Mehrere Adressen (Load Balancing): die erste zählt, RTT über alle Proben.
	if hops[3].IP != "88.79.26.24" || rtt(hops[3]) != 2.6 {
		t.Errorf("Hop 4 (zwei Adressen): %+v rtt=%v", hops[3], rtt(hops[3]))
	}
}

func ptr[T any](v T) *T { return &v }

func hop(n int, ip string, lat, lon *float64, city string) model.TraceHop {
	h := model.TraceHop{HopNumber: n, Responded: ip != "", Latitude: lat, Longitude: lon}
	if ip != "" {
		h.IPAddress = ptr(ip)
	}
	if city != "" {
		h.City = ptr(city)
	}
	return h
}

func TestInterpolate(t *testing.T) {
	hops := []model.TraceHop{
		hop(1, "10.0.0.1", nil, nil, ""),                     // privat, kein Spender in /24 → nächster Hop
		hop(2, "80.1.1.1", ptr(50.0), ptr(8.0), "Frankfurt"), // Spender
		hop(3, "", nil, nil, ""),                             // Timeout: bleibt leer
		hop(4, "90.2.2.2", ptr(52.0), ptr(13.0), "Berlin"),
		hop(5, "80.1.1.9", nil, nil, ""), // gleiches /24 wie Hop 2 → Frankfurt, obwohl Hop 4 näher ist
	}
	Interpolate(hops, rand.New(rand.NewPCG(1, 2)))

	near := func(h model.TraceHop, lat, lon float64) bool {
		return h.HasLocation() && abs(*h.Latitude-lat) <= jitterDegrees && abs(*h.Longitude-lon) <= jitterDegrees
	}
	if !hops[0].GeoIPInterpolated || !near(hops[0], 50, 8) || *hops[0].City != "? Frankfurt" {
		t.Errorf("Hop 1: %+v", hops[0])
	}
	if hops[2].HasLocation() || hops[2].GeoIPInterpolated {
		t.Errorf("Timeout-Hop darf nicht interpoliert werden: %+v", hops[2])
	}
	if !hops[4].GeoIPInterpolated || !near(hops[4], 50, 8) {
		t.Errorf("Hop 5 sollte vom /24-Nachbarn (Frankfurt) übernehmen: lat=%v lon=%v", *hops[4].Latitude, *hops[4].Longitude)
	}
	if hops[1].GeoIPInterpolated || *hops[1].Latitude != 50 {
		t.Errorf("Spender darf nicht verändert werden: %+v", hops[1])
	}

	none := []model.TraceHop{hop(1, "10.0.0.1", nil, nil, "")}
	Interpolate(none, rand.New(rand.NewPCG(1, 2)))
	if none[0].HasLocation() {
		t.Error("ohne Spender dürfen keine Koordinaten entstehen")
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// TestHelperProcess ersetzt tracert/traceroute: Die Test-Binary gibt das
// Fixture aus TRACE_HELPER_FIXTURE zeilenweise mit Verzögerung aus.
func TestHelperProcess(t *testing.T) {
	fixture := os.Getenv("TRACE_HELPER_FIXTURE")
	if fixture == "" {
		return
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		os.Exit(2)
	}
	for _, line := range strings.SplitAfter(string(data), "\n") {
		fmt.Print(line)
		time.Sleep(20 * time.Millisecond)
	}
	os.Exit(0)
}

func newHelperTracer(t *testing.T, fixture string) *Tracer {
	t.Helper()
	abs, _ := filepath.Abs(filepath.Join("testdata", fixture))
	t.Setenv("TRACE_HELPER_FIXTURE", abs)
	tr := NewTracer(nil, os.Args[0], "-test.run=^TestHelperProcess$", "--")
	tr.lookupAddr = func(_ context.Context, ip string) ([]string, error) {
		if ip == "8.8.8.8" {
			return []string{"dns.google."}, nil
		}
		return nil, errors.New("kein PTR")
	}
	return tr
}

func TestTracerRun(t *testing.T) {
	tr := newHelperTracer(t, "traceroute_linux.txt")
	var live []int
	res := tr.Run(context.Background(), "8.8.8.8", DefaultOptions(), func(h model.TraceHop) { live = append(live, h.HopNumber) })

	if !res.Completed || res.ErrorMessage != nil {
		t.Fatalf("nicht vollständig: %v", *res.ErrorMessage)
	}
	// Der Trace endet am Ziel (Hop 5); die Zeile danach wird nicht mehr gelesen.
	if res.TotalHops != 5 || len(res.Hops) != 5 || len(live) != 5 {
		t.Fatalf("Hops: total=%d, gespeichert=%d, live=%d", res.TotalHops, len(res.Hops), len(live))
	}
	last := res.Hops[4]
	if last.Hostname == nil || *last.Hostname != "dns.google" || !last.Responded {
		t.Errorf("Ziel-Hop: %+v", last)
	}
	if res.Hops[1].Responded || res.Hops[1].IPAddress != nil {
		t.Errorf("Timeout-Hop: %+v", res.Hops[1])
	}
	if res.TotalRTTMs == nil || *res.TotalRTTMs != 4.54 {
		t.Errorf("total_rtt_ms = %v, erwartet RTT des Ziels (4.54)", res.TotalRTTMs)
	}
	if res.DestinationIP == nil || *res.DestinationIP != "8.8.8.8" || res.StartedAt == nil || res.CompletedAt == nil {
		t.Errorf("Metadaten: %+v", res)
	}
}

func TestTracerMissingCommand(t *testing.T) {
	tr := NewTracer(nil, filepath.Join(t.TempDir(), "kein-traceroute"))
	res := tr.Run(context.Background(), "127.0.0.1", DefaultOptions(), nil)
	if res.Completed || res.ErrorMessage == nil || !strings.Contains(*res.ErrorMessage, "nicht gefunden") {
		t.Errorf("Fehler erwartet: %+v", res)
	}
}

func TestGeoIPLookup(t *testing.T) {
	// Nutzt die im Repository liegende GeoLite2-Datenbank, sofern vorhanden.
	geo, _, err := OpenGeoIP(filepath.Join("..", "..", "geoip", "GeoLite2-City.mmdb"))
	if err != nil {
		t.Skip("GeoLite2-City.mmdb nicht vorhanden")
	}
	defer geo.Close()

	if loc := geo.Lookup("8.8.8.8"); loc == nil || loc.CountryCode != "US" {
		t.Errorf("8.8.8.8: %+v", loc)
	}
	if loc := geo.Lookup("10.10.80.1"); loc != nil {
		t.Errorf("private Adresse darf keinen Standort haben: %+v", loc)
	}
	var none *GeoIP
	if none.Lookup("8.8.8.8") != nil {
		t.Error("nil-GeoIP muss nil liefern")
	}
}

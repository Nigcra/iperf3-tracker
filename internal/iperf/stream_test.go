package iperf

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"iperf3-tracker/internal/model"
)

// readFixture liefert die Intervall- und Abschlussdaten eines echten
// iperf3-Mitschnitts (iperf 3.21, `--json-stream`, gegen 127.0.0.1).
func readFixture(t *testing.T, name string) (intervals []intervalData, end *endData) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var ev event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		switch ev.Event {
		case "interval":
			var d intervalData
			if err := json.Unmarshal(ev.Data, &d); err != nil {
				t.Fatal(err)
			}
			intervals = append(intervals, d)
		case "end":
			end = &endData{}
			if err := json.Unmarshal(ev.Data, end); err != nil {
				t.Fatal(err)
			}
		}
	}
	return intervals, end
}

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }

func eqF(a, b *float64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
func eqI(a, b *int64) bool   { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func checkResult(t *testing.T, got, want model.TestResult) {
	t.Helper()
	fields := []struct {
		name      string
		got, want *float64
	}{
		{"download_bandwidth_mbps", got.DownloadBandwidthMbps, want.DownloadBandwidthMbps},
		{"download_jitter_ms", got.DownloadJitterMs, want.DownloadJitterMs},
		{"download_packet_loss_percent", got.DownloadPacketLossPercent, want.DownloadPacketLossPercent},
		{"upload_bandwidth_mbps", got.UploadBandwidthMbps, want.UploadBandwidthMbps},
		{"upload_jitter_ms", got.UploadJitterMs, want.UploadJitterMs},
		{"upload_packet_loss_percent", got.UploadPacketLossPercent, want.UploadPacketLossPercent},
		{"cpu_percent", got.CPUPercent, want.CPUPercent},
	}
	for _, f := range fields {
		if !eqF(f.got, f.want) {
			t.Errorf("%s = %v, erwartet %v", f.name, deref(f.got), deref(f.want))
		}
	}
	if !eqI(got.DownloadBytes, want.DownloadBytes) || !eqI(got.UploadBytes, want.UploadBytes) || !eqI(got.Retransmits, want.Retransmits) {
		t.Errorf("Bytes/Retransmits: got %v/%v/%v, want %v/%v/%v",
			derefI(got.DownloadBytes), derefI(got.UploadBytes), derefI(got.Retransmits),
			derefI(want.DownloadBytes), derefI(want.UploadBytes), derefI(want.Retransmits))
	}
}

func deref(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func derefI(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func TestEndResult(t *testing.T) {
	cases := []struct {
		fixture string
		dir     model.Direction
		want    model.TestResult
	}{
		{"tcp_download.jsonl", model.DirectionDownload, model.TestResult{
			DownloadBandwidthMbps: f64(30625.96), DownloadBytes: i64(7665876992), CPUPercent: f64(75.42),
		}},
		{"tcp_upload.jsonl", model.DirectionUpload, model.TestResult{
			UploadBandwidthMbps: f64(31741.85), UploadBytes: i64(7949647872), CPUPercent: f64(96.41),
		}},
		{"tcp_bidir_p2.jsonl", model.DirectionBidirectional, model.TestResult{
			UploadBandwidthMbps: f64(44140.63), UploadBytes: i64(11220680704),
			DownloadBandwidthMbps: f64(47694.28), DownloadBytes: i64(12124028928),
			CPUPercent: f64(254.43),
		}},
		{"udp_download.jsonl", model.DirectionDownload, model.TestResult{
			DownloadBandwidthMbps: f64(1.31), DownloadBytes: i64(327475),
			DownloadJitterMs: f64(0.05), DownloadPacketLossPercent: f64(0), CPUPercent: f64(0),
		}},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			_, end := readFixture(t, c.fixture)
			if end == nil {
				t.Fatal("kein end-Ereignis")
			}
			got, ok := endResult(*end, c.dir)
			if !ok {
				t.Fatal("endResult: keine Ergebnisse")
			}
			checkResult(t, got, c.want)
		})
	}
}

func TestEndResultRetransmits(t *testing.T) {
	// Linux-iperf3 liefert retransmits beim Sender; bei bidir werden beide Richtungen addiert.
	var d endData
	if err := json.Unmarshal([]byte(`{
		"sum_sent": {"bytes": 1, "bits_per_second": 1e6, "retransmits": 3},
		"sum_received": {"bytes": 1, "bits_per_second": 1e6},
		"sum_sent_bidir_reverse": {"bytes": 2, "bits_per_second": 2e6, "retransmits": 4},
		"sum_received_bidir_reverse": {"bytes": 2, "bits_per_second": 2e6}}`), &d); err != nil {
		t.Fatal(err)
	}
	got, ok := endResult(d, model.DirectionBidirectional)
	if !ok || got.Retransmits == nil || *got.Retransmits != 7 {
		t.Fatalf("Retransmits = %v, ok = %v", derefI(got.Retransmits), ok)
	}
	if got, ok := endResult(d, model.DirectionUpload); !ok || *got.Retransmits != 3 {
		t.Fatalf("Upload-Retransmits = %v", derefI(got.Retransmits))
	}
}

func TestEndResultMissingDirection(t *testing.T) {
	_, end := readFixture(t, "tcp_download.jsonl")
	if _, ok := endResult(*end, model.DirectionBidirectional); ok {
		t.Error("Bidir-Auswertung eines Download-Tests darf nicht ok sein")
	}
}

func TestIntervalRates(t *testing.T) {
	iv, _ := readFixture(t, "tcp_bidir_p2.jsonl")
	if len(iv) == 0 {
		t.Fatal("keine Intervalle")
	}
	r := intervalRates(iv[0], model.DirectionBidirectional)
	if !eqF(r.UploadMbps, f64(43912.55)) || !eqF(r.DownloadMbps, f64(45041.63)) {
		t.Errorf("bidir: up=%v down=%v", deref(r.UploadMbps), deref(r.DownloadMbps))
	}

	iv, _ = readFixture(t, "tcp_download.jsonl")
	r = intervalRates(iv[0], model.DirectionDownload)
	if r.UploadMbps != nil || !eqF(r.DownloadMbps, f64(30397.01)) {
		t.Errorf("download: up=%v down=%v", deref(r.UploadMbps), deref(r.DownloadMbps))
	}
}

func TestArgs(t *testing.T) {
	test := &model.Test{Duration: 5, ParallelStreams: 2, Protocol: model.ProtocolUDP, Direction: model.DirectionDownload, UDPBandwidthMbps: f64(250.5)}
	got := Args("srv.example.org", 5201, test)
	want := []string{"-c", "srv.example.org", "-p", "5201", "-t", "5", "-P", "2", "-i", "1", "--json-stream", "--forceflush", "-u", "-b", "250.5M", "-R"}
	if len(got) != len(want) {
		t.Fatalf("Args = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Args = %v, erwartet %v", got, want)
		}
	}

	test.Protocol, test.Direction = model.ProtocolTCP, model.DirectionUpload
	// Bei TCP wird keine Zielbandbreite übergeben, auch wenn eine gesetzt ist.
	for _, a := range Args("h", 1, test) {
		if a == "-R" || a == "--bidir" || a == "-u" || a == "-b" {
			t.Errorf("Upload/TCP darf %s nicht enthalten", a)
		}
	}
}

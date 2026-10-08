package iperf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

// TestHelperProcess ist kein echter Test: Der Runner startet die Test-Binary
// mit diesem Testnamen als Ersatz für iperf3. Sie gibt das Fixture aus
// IPERF_HELPER_FIXTURE zeilenweise aus, protokolliert ihre Argumente nach
// IPERF_HELPER_ARGS und endet mit dem Code aus IPERF_HELPER_EXIT.
func TestHelperProcess(t *testing.T) {
	fixture := os.Getenv("IPERF_HELPER_FIXTURE")
	if fixture == "" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if p := os.Getenv("IPERF_HELPER_ARGS"); p != "" {
		os.WriteFile(p, []byte(strings.Join(args, " ")), 0644)
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, line := range strings.SplitAfter(string(data), "\n") {
		fmt.Print(line)
		time.Sleep(50 * time.Millisecond)
	}
	code, _ := strconv.Atoi(os.Getenv("IPERF_HELPER_EXIT"))
	os.Exit(code)
}

type runnerEnv struct {
	store  *store.Store
	runner *Runner
	server *model.Server
	args   string // Datei mit den Argumenten des letzten Aufrufs
}

func newRunnerEnv(t *testing.T, fixture string, exitCode int) *runnerEnv {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	st := store.New(conn)

	sv := &model.Server{ServerSettings: model.DefaultServerSettings()}
	sv.Name, sv.Host = "lokal", "127.0.0.1"
	if err := st.CreateServer(context.Background(), sv); err != nil {
		t.Fatal(err)
	}

	env := &runnerEnv{store: st, server: sv, args: filepath.Join(t.TempDir(), "args.txt")}
	abs, _ := filepath.Abs(filepath.Join("testdata", fixture))
	t.Setenv("IPERF_HELPER_FIXTURE", abs)
	t.Setenv("IPERF_HELPER_ARGS", env.args)
	t.Setenv("IPERF_HELPER_EXIT", strconv.Itoa(exitCode))

	env.runner = NewRunner(st, os.Args[0], "-test.run=^TestHelperProcess$", "--")
	t.Cleanup(env.runner.Stop)
	return env
}

func (e *runnerEnv) submit(t *testing.T, dir model.Direction) *model.Test {
	t.Helper()
	test := &model.Test{ServerID: e.server.ID, Protocol: model.ProtocolTCP, Direction: dir, Duration: 2, ParallelStreams: 1}
	if err := e.store.CreateTest(context.Background(), test); err != nil {
		t.Fatal(err)
	}
	e.runner.Submit(test)
	return test
}

// waitDone wartet, bis der Test einen Endstatus hat, und liefert ihn mit Rohausgabe.
func (e *runnerEnv) waitDone(t *testing.T, id int64) *model.TestDetail {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		d, err := e.store.TestDetailByID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if d.Status == model.StatusCompleted || d.Status == model.StatusFailed {
			return d
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Test %d nicht rechtzeitig beendet", id)
	return nil
}

func TestRunnerCompleted(t *testing.T) {
	env := newRunnerEnv(t, "tcp_download.jsonl", 0)
	test := env.submit(t, model.DirectionDownload)

	d := env.waitDone(t, test.ID)
	if d.Status != model.StatusCompleted || d.ErrorMessage != nil {
		t.Fatalf("Status %s, Fehler %v", d.Status, d.ErrorMessage)
	}
	if !eqF(d.DownloadBandwidthMbps, f64(30625.96)) || d.UploadBandwidthMbps != nil {
		t.Errorf("Download %v, Upload %v", deref(d.DownloadBandwidthMbps), deref(d.UploadBandwidthMbps))
	}
	if d.StartedAt == nil || d.CompletedAt == nil || d.CompletedAt.Before(*d.StartedAt) {
		t.Errorf("Zeitstempel: %v – %v", d.StartedAt, d.CompletedAt)
	}
	if d.RawOutput == nil || !strings.Contains(*d.RawOutput, `"event":"end"`) {
		t.Error("Rohausgabe fehlt")
	}

	args, _ := os.ReadFile(env.args)
	if !strings.Contains(string(args), "-c 127.0.0.1 -p 5201 -t 2 -P 1") || !strings.Contains(string(args), "-R") {
		t.Errorf("iperf3-Argumente: %s", args)
	}

	// Der Live-Status bleibt nach Testende mit den Endwerten abrufbar.
	l, ok := env.runner.Live(test.ID)
	if !ok || l.Status != model.StatusCompleted || l.Progress != 100 || l.CurrentDownloadMbps != 30625.96 {
		t.Errorf("Live nach Ende: %+v, ok=%v", l, ok)
	}
}

func TestRunnerIperfError(t *testing.T) {
	env := newRunnerEnv(t, "error_refused.jsonl", 1)
	test := env.submit(t, model.DirectionUpload)

	d := env.waitDone(t, test.ID)
	if d.Status != model.StatusFailed || d.ErrorMessage == nil || !strings.Contains(*d.ErrorMessage, "Connection refused") {
		t.Fatalf("Status %s, Fehler %v", d.Status, deref2(d.ErrorMessage))
	}
	if l, _ := env.runner.Live(test.ID); l.Status != model.StatusFailed {
		t.Errorf("Live-Status %s", l.Status)
	}
}

func TestRunnerSequential(t *testing.T) {
	env := newRunnerEnv(t, "tcp_upload.jsonl", 0)
	first := env.submit(t, model.DirectionUpload)
	second := env.submit(t, model.DirectionUpload)

	a := env.waitDone(t, first.ID)
	b := env.waitDone(t, second.ID)
	if a.Status != model.StatusCompleted || b.Status != model.StatusCompleted {
		t.Fatalf("Status %s / %s", a.Status, b.Status)
	}
	// Nacheinander, ohne die 10 s Wartezeit des Python-Backends dazwischen.
	x, y := a, b
	if y.StartedAt.Before(*x.StartedAt) {
		x, y = y, x
	}
	if y.StartedAt.Before(*x.CompletedAt) {
		t.Errorf("Tests liefen parallel: %v < %v", y.StartedAt, x.CompletedAt)
	}
	if gap := y.StartedAt.Sub(*x.CompletedAt); gap > 2*time.Second {
		t.Errorf("Lücke zwischen den Tests zu groß: %s", gap)
	}
}

func TestRunnerMissingBinary(t *testing.T) {
	env := newRunnerEnv(t, "tcp_upload.jsonl", 0)
	env.runner.command = []string{filepath.Join(t.TempDir(), "kein-iperf3")}
	test := env.submit(t, model.DirectionUpload)

	d := env.waitDone(t, test.ID)
	if d.Status != model.StatusFailed || d.ErrorMessage == nil || !strings.Contains(*d.ErrorMessage, "nicht gefunden") {
		t.Fatalf("Status %s, Fehler %v", d.Status, deref2(d.ErrorMessage))
	}
}

func deref2(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

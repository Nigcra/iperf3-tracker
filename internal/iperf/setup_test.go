package iperf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMain lässt die Test-Binary als gefälschtes iperf3 auftreten: Mit
// FAKE_IPERF3_VERSION gibt sie eine Versionszeile aus und endet.
func TestMain(m *testing.M) {
	if v := os.Getenv("FAKE_IPERF3_VERSION"); v != "" {
		fmt.Printf("iperf %s (cJSON 1.7.15)\n", v)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestInstallCommands(t *testing.T) {
	look := func(tools ...string) func(string) (string, error) {
		return func(name string) (string, error) {
			for _, t := range tools {
				if t == name {
					return "/usr/bin/" + name, nil
				}
			}
			return "", errors.New("nicht gefunden")
		}
	}
	cases := []struct {
		name    string
		goos    string
		tools   []string
		root    bool
		want    [][]string
		wantErr string
	}{
		{"windows winget", "windows", []string{"winget"}, false,
			[][]string{{"winget", "install", "--id", "ar51an.iPerf3", "-e", "--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}}, ""},
		{"windows ohne winget", "windows", nil, false, nil, "winget ist nicht verfügbar"},
		{"debian als root", "linux", []string{"apt-get", "sudo"}, true,
			[][]string{{"apt-get", "update"}, {"apt-get", "install", "-y", "iperf3"}}, ""},
		{"debian mit sudo", "linux", []string{"apt-get", "sudo"}, false,
			[][]string{{"sudo", "-n", "apt-get", "update"}, {"sudo", "-n", "apt-get", "install", "-y", "iperf3"}}, ""},
		{"fedora", "linux", []string{"dnf", "yum"}, true, [][]string{{"dnf", "install", "-y", "iperf3"}}, ""},
		{"alpine", "linux", []string{"apk"}, true, [][]string{{"apk", "add", "--no-cache", "iperf3"}}, ""},
		{"ohne root und sudo", "linux", []string{"zypper"}, false, nil, "Root-Rechte nötig"},
		{"kein Paketmanager", "linux", nil, true, nil, "kein unterstützter Paketmanager"},
		{"macOS", "darwin", []string{"brew"}, false, [][]string{{"brew", "install", "iperf3"}}, ""},
		{"unbekannt", "plan9", nil, true, nil, "nicht unterstützt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := installCommands(c.goos, look(c.tools...), c.root)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("Fehler %v, erwartet %q", err, c.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v, %v\nwant %v", got, err, c.want)
			}
		})
	}
}

// isolateSearch sorgt dafür, dass FindBinary nur dir durchsucht.
func isolateSearch(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("ProgramFiles", dir)
	return dir
}

// placeFakeIperf kopiert die Test-Binary als iperf3 nach dir.
func placeFakeIperf(t *testing.T, dir string) string {
	t.Helper()
	name := "iperf3"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	src, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst := filepath.Join(dir, name)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(f, src); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return dst
}

func waitInstalled(t *testing.T, r *Runner) Status {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for r.Status().Installing {
		if time.Now().After(deadline) {
			t.Fatal("Installation nicht rechtzeitig beendet")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return r.Status()
}

// stubInstall ersetzt Installationsplanung und Installation. Ist plan nil,
// gilt iperf3 als installierbar per "fake install iperf3".
func stubInstall(t *testing.T, planErr error, f func(ctx context.Context) error) *int {
	t.Helper()
	calls := 0
	origPlan, origFunc := installPlan, installFunc
	installPlan = func() ([][]string, error) {
		if planErr != nil {
			return nil, planErr
		}
		return [][]string{{"fake", "install", "iperf3"}}, nil
	}
	installFunc = func(ctx context.Context) error { calls++; return f(ctx) }
	t.Cleanup(func() { installPlan, installFunc = origPlan, origFunc })
	return &calls
}

func TestInstallOnRequest(t *testing.T) {
	dir := isolateSearch(t)
	t.Setenv("FAKE_IPERF3_VERSION", "3.21")
	release := make(chan struct{})
	calls := stubInstall(t, nil, func(context.Context) error { <-release; placeFakeIperf(t, dir); return nil })

	r := NewRunner(nil, "iperf3")
	t.Cleanup(r.Stop)
	r.Prepare("", false)
	st := r.Status()
	if st.Available || st.Installing || !st.Installable || st.InstallCommand != "fake install iperf3" || *calls != 0 {
		t.Fatalf("ohne Auto-Installation: %+v, Aufrufe %d", st, *calls)
	}

	if err := r.InstallAsync(); err != nil {
		t.Fatal(err)
	}
	if err := r.InstallAsync(); !errors.Is(err, ErrInstallRunning) {
		t.Errorf("zweiter Aufruf: %v, erwartet ErrInstallRunning", err)
	}
	close(release)
	st = waitInstalled(t, r)
	if !st.Available || st.Version != "3.21" || st.Installable || *calls != 1 || r.commandLine()[0] != st.Path {
		t.Fatalf("nach Installation: %+v, Aufrufe %d", st, *calls)
	}
	if err := r.InstallAsync(); !errors.Is(err, ErrNotInstallable) {
		t.Errorf("bereits vorhanden: %v, erwartet ErrNotInstallable", err)
	}
}

func TestPrepareAutoInstall(t *testing.T) {
	dir := isolateSearch(t)
	t.Setenv("FAKE_IPERF3_VERSION", "3.21")
	calls := stubInstall(t, nil, func(context.Context) error { placeFakeIperf(t, dir); return nil })

	r := NewRunner(nil, "iperf3")
	t.Cleanup(r.Stop)
	r.Prepare("", true)
	if st := waitInstalled(t, r); !st.Available || *calls != 1 {
		t.Fatalf("auto_install: %+v, Aufrufe %d", st, *calls)
	}
}

func TestInstallFailureCanBeRetried(t *testing.T) {
	isolateSearch(t)
	stubInstall(t, nil, func(context.Context) error { return errors.New("winget: Zugriff verweigert") })

	r := NewRunner(nil, "iperf3")
	t.Cleanup(r.Stop)
	r.Prepare("", false)
	if err := r.InstallAsync(); err != nil {
		t.Fatal(err)
	}
	st := waitInstalled(t, r)
	if st.Available || !st.Installable || !strings.Contains(st.Error, "Installation fehlgeschlagen") || !strings.Contains(st.Error, "Zugriff verweigert") {
		t.Fatalf("Status: %+v", st)
	}
	if err := r.InstallAsync(); err != nil {
		t.Errorf("erneuter Versuch: %v", err)
	}
	waitInstalled(t, r)
}

func TestNotInstallable(t *testing.T) {
	dir := isolateSearch(t)
	calls := stubInstall(t, nil, func(context.Context) error { return nil })
	r := NewRunner(nil, "iperf3")
	t.Cleanup(r.Stop)

	// Fest konfigurierter Pfad wird nie ersetzt, auch nicht mit auto_install.
	r.Prepare(filepath.Join(dir, "gibt-es-nicht.exe"), true)
	if st := r.Status(); st.Installable || st.Installing || *calls != 0 || !errors.Is(r.InstallAsync(), ErrNotInstallable) {
		t.Fatalf("konfigurierter Pfad: %+v, Aufrufe %d", st, *calls)
	}

	// Zu alte Version: nur melden.
	placeFakeIperf(t, dir)
	t.Setenv("FAKE_IPERF3_VERSION", "3.12")
	r.Prepare("", true)
	if st := r.Status(); st.Installable || st.Installing || *calls != 0 || !strings.Contains(st.Error, "zu alt") {
		t.Fatalf("zu alt: %+v, Aufrufe %d", st, *calls)
	}
}

func TestNoPackageManager(t *testing.T) {
	isolateSearch(t)
	stubInstall(t, errors.New("kein unterstützter Paketmanager gefunden"), func(context.Context) error { return nil })
	r := NewRunner(nil, "iperf3")
	t.Cleanup(r.Stop)
	r.Prepare("", true)
	if st := r.Status(); st.Installable || st.Installing || !strings.Contains(st.Error, "kein unterstützter Paketmanager") {
		t.Fatalf("Status: %+v", st)
	}
}

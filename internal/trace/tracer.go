package trace

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

// maxDuration begrenzt eine Routenverfolgung insgesamt. Bereits ermittelte
// Hops bleiben bei Überschreitung erhalten.
const maxDuration = 90 * time.Second

// reverseDNSTimeout begrenzt die Namensauflösung je Hop.
const reverseDNSTimeout = 1500 * time.Millisecond

// Options steuern eine Routenverfolgung.
type Options struct {
	MaxHops int // maximale Anzahl Hops
	WaitSec int // Wartezeit je Probe in Sekunden
	Probes  int // Proben je Hop (nur traceroute; tracert sendet immer 3)
}

// DefaultOptions: 30 Hops, 2 s Wartezeit je Probe, 3 Proben.
func DefaultOptions() Options { return Options{MaxHops: 30, WaitSec: 2, Probes: 3} }

// HopFunc wird für jeden Hop aufgerufen, sobald er ermittelt ist (Live-Ansicht).
type HopFunc func(model.TraceHop)

// Tracer führt Routenverfolgungen aus.
type Tracer struct {
	geo *GeoIP
	// command ersetzt tracert/traceroute (Tests); die Ziel-IP wird angehängt.
	command    []string
	lookupAddr func(ctx context.Context, ip string) ([]string, error)
}

// NewTracer erstellt einen Tracer. geo darf nil sein (dann ohne Standorte).
// command ersetzt optional tracert/traceroute; die Ziel-IP wird angehängt.
func NewTracer(geo *GeoIP, command ...string) *Tracer {
	t := &Tracer{geo: geo, lookupAddr: net.DefaultResolver.LookupAddr}
	if len(command) > 0 {
		t.command = command
	}
	return t
}

// Run verfolgt die Route zu destination. Die Verfolgung endet, sobald das
// Ziel antwortet. Fehler werden im Trace vermerkt (Completed = false).
func (t *Tracer) Run(ctx context.Context, destination string, opts Options, onHop HopFunc) *model.Trace {
	started := db.Now()
	tr := &model.Trace{DestinationHost: destination, StartedAt: &started, Hops: []model.TraceHop{}}
	defer func() {
		done := db.Now()
		tr.CompletedAt = &done
		tr.TotalHops = len(tr.Hops)
	}()

	ctx, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()

	destIP, err := resolveIPv4(ctx, destination)
	if err != nil {
		msg := "Ziel konnte nicht aufgelöst werden: " + err.Error()
		tr.ErrorMessage = &msg
		return tr
	}
	tr.DestinationIP = &destIP
	if src := sourceIP(destIP); src != "" {
		tr.SourceIP = &src
	}

	err = t.trace(ctx, destIP, opts, func(raw rawHop) bool {
		hop := t.enrich(ctx, raw)
		tr.Hops = append(tr.Hops, hop)
		if onHop != nil {
			onHop(hop)
		}
		return raw.IP == destIP
	})

	Interpolate(tr.Hops, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	// RTT bis zum Ziel = Laufzeit des letzten antwortenden Hops.
	for i := len(tr.Hops) - 1; i >= 0; i-- {
		if tr.Hops[i].RTTMs != nil {
			tr.TotalRTTMs = tr.Hops[i].RTTMs
			break
		}
	}
	if err != nil {
		msg := err.Error()
		tr.ErrorMessage = &msg
	} else {
		tr.Completed = true
	}
	return tr
}

// trace startet das Werkzeug und übergibt jede Hop-Zeile an handle. Liefert
// handle true, wird das Werkzeug beendet (Ziel erreicht).
func (t *Tracer) trace(ctx context.Context, destIP string, o Options, handle func(rawHop) bool) error {
	name, args := t.commandLine(destIP, o)
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s nicht gefunden – bitte installieren", name)
		}
		return fmt.Errorf("%s konnte nicht gestartet werden: %w", name, err)
	}

	reached := false
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		h, ok := parseHopLine(sc.Text())
		if !ok {
			continue
		}
		if handle(h) {
			reached = true
			cmd.Process.Kill()
			break
		}
	}
	waitErr := cmd.Wait()

	switch {
	case reached:
		return nil
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Errorf("Zeitüberschreitung nach %s", maxDuration)
	case ctx.Err() != nil:
		return errors.New("Abgebrochen")
	case waitErr != nil:
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return errors.New(msg)
		}
		return fmt.Errorf("%s beendet mit Fehler: %w", name, waitErr)
	}
	return nil
}

func (t *Tracer) commandLine(destIP string, o Options) (string, []string) {
	if t.command != nil {
		return t.command[0], append(append([]string{}, t.command[1:]...), destIP)
	}
	if runtime.GOOS == "windows" {
		return "tracert", []string{"-d", "-h", strconv.Itoa(o.MaxHops), "-w", strconv.Itoa(o.WaitSec * 1000), destIP}
	}
	return "traceroute", []string{"-n", "-m", strconv.Itoa(o.MaxHops), "-w", strconv.Itoa(o.WaitSec), "-q", strconv.Itoa(o.Probes), destIP}
}

// enrich ergänzt Hostname und Standort eines beantworteten Hops.
func (t *Tracer) enrich(ctx context.Context, raw rawHop) model.TraceHop {
	hop := model.TraceHop{HopNumber: raw.Number, Responded: raw.IP != ""}
	if !hop.Responded {
		return hop
	}
	ip := raw.IP
	hop.IPAddress = &ip
	hop.RTTMs = raw.RTTMs

	dnsCtx, cancel := context.WithTimeout(ctx, reverseDNSTimeout)
	if names, err := t.lookupAddr(dnsCtx, ip); err == nil && len(names) > 0 {
		name := strings.TrimSuffix(names[0], ".")
		hop.Hostname = &name
	}
	cancel()

	if loc := t.geo.Lookup(ip); loc != nil {
		lat, lon := loc.Latitude, loc.Longitude
		hop.Latitude, hop.Longitude = &lat, &lon
		hop.City = nonEmpty(loc.City)
		hop.Country = nonEmpty(loc.Country)
		hop.CountryCode = nonEmpty(loc.CountryCode)
	}
	return hop
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// resolveIPv4 liefert die IPv4-Adresse von host. Die Verfolgung läuft auf die
// Adresse, damit tracert/traceroute dasselbe Ziel nutzen wie der Vergleich
// "Ziel erreicht".
func resolveIPv4(ctx context.Context, host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
		return "", errors.New("nur IPv4 wird unterstützt")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return "", err
	}
	return ips[0].String(), nil
}

// sourceIP ermittelt die lokale Adresse, über die destIP erreicht wird.
// Ein UDP-"Connect" sendet dabei keine Pakete.
func sourceIP(destIP string) string {
	conn, err := net.Dial("udp4", net.JoinHostPort(destIP, "80"))
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

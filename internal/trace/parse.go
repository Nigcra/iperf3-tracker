// Package trace führt Routenverfolgungen aus (tracert unter Windows,
// traceroute sonst), ergänzt Standortdaten aus der GeoIP-Datenbank und
// interpoliert fehlende Koordinaten.
package trace

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	hopLineRe = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	ipv4Re    = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	rttRe     = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*ms\b`)
)

// rawHop ist eine geparste Hop-Zeile vor der Anreicherung.
type rawHop struct {
	Number int
	IP     string   // leer, wenn der Hop nicht geantwortet hat
	RTTMs  *float64 // Mittelwert aller gemessenen Laufzeiten
}

// parseHopLine wertet eine Zeile von tracert oder traceroute (jeweils ohne
// Namensauflösung, -d bzw. -n) aus. ok ist false für Kopf- und Fußzeilen.
//
// Ein Hop gilt als beantwortet, sobald eine IP-Adresse in der Zeile steht –
// unabhängig von der Sprache der Timeout-Meldung und auch bei teilweisen
// Timeouts wie "12 ms  *  14 ms  1.2.3.4". Bei mehreren Adressen (Load
// Balancing) zählt die erste. "<1 ms" wird als 1 ms gewertet.
func parseHopLine(line string) (h rawHop, ok bool) {
	m := hopLineRe.FindStringSubmatch(line)
	if m == nil {
		return h, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return h, false
	}
	h.Number = n
	rest := m[2]

	h.IP = ipv4Re.FindString(rest)
	if h.IP == "" {
		return h, true
	}

	var sum float64
	rtts := rttRe.FindAllStringSubmatch(rest, -1)
	for _, r := range rtts {
		v, _ := strconv.ParseFloat(r[1], 64)
		sum += v
	}
	if len(rtts) > 0 {
		avg := round2(sum / float64(len(rtts)))
		h.RTTMs = &avg
	}
	return h, true
}

// subnet24 liefert die ersten drei Oktette einer IPv4-Adresse ("1.2.3") oder "".
func subnet24(ip string) string {
	i := strings.LastIndexByte(ip, '.')
	if i < 0 || strings.Count(ip, ".") != 3 {
		return ""
	}
	return ip[:i]
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

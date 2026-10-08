package trace

import (
	"net"
	"os"

	"github.com/oschwald/geoip2-golang"
)

// GeoIP schlägt Standorte in einer GeoLite2-City-Datenbank nach. Ein nil-
// *GeoIP ist gültig und liefert keine Standorte.
type GeoIP struct {
	reader *geoip2.Reader
}

// Location ist der Standort einer IP-Adresse.
type Location struct {
	Latitude, Longitude float64
	City, Country       string
	CountryCode         string
}

// OpenGeoIP öffnet die erste vorhandene Datenbank aus paths.
func OpenGeoIP(paths ...string) (*GeoIP, string, error) {
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		r, err := geoip2.Open(p)
		if err != nil {
			return nil, p, err
		}
		return &GeoIP{reader: r}, p, nil
	}
	return nil, "", os.ErrNotExist
}

// Close schließt die Datenbank.
func (g *GeoIP) Close() error {
	if g == nil {
		return nil
	}
	return g.reader.Close()
}

// Lookup liefert den Standort von ip oder nil, wenn er unbekannt ist (z. B.
// bei privaten Adressen).
func (g *GeoIP) Lookup(ip string) *Location {
	parsed := net.ParseIP(ip)
	if g == nil || parsed == nil {
		return nil
	}
	rec, err := g.reader.City(parsed)
	if err != nil || (rec.Location.Latitude == 0 && rec.Location.Longitude == 0) {
		return nil
	}
	return &Location{
		Latitude:    rec.Location.Latitude,
		Longitude:   rec.Location.Longitude,
		City:        rec.City.Names["en"],
		Country:     rec.Country.Names["en"],
		CountryCode: rec.Country.IsoCode,
	}
}

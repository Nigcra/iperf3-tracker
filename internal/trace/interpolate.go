package trace

import (
	"math/rand/v2"

	"iperf3-tracker/internal/model"
)

const (
	// sameSubnetRange ist der Suchradius (in Hops) für einen Spender im selben /24-Netz.
	sameSubnetRange = 5
	// jitterDegrees verschiebt interpolierte Punkte zufällig (ca. ±5 km), damit
	// sie auf der Karte nicht exakt auf dem Spender liegen.
	jitterDegrees = 0.05
)

// Interpolate ergänzt Koordinaten für beantwortete Hops, die die GeoIP-
// Datenbank nicht kennt. Bevorzugt wird ein Hop im selben /24-Netz innerhalb
// von ±5 Hops, sonst der nächstgelegene Hop mit Koordinaten (jeweils erst
// rückwärts, dann vorwärts). Übernommene Daten werden mit GeoIPInterpolated
// markiert, die Stadt mit vorangestelltem "? ".
func Interpolate(hops []model.TraceHop, rnd *rand.Rand) {
	for i := range hops {
		h := &hops[i]
		if h.HasLocation() || !h.Responded || h.IPAddress == nil {
			continue
		}
		donor := findDonor(hops, i)
		if donor == nil {
			continue
		}
		lat := *donor.Latitude + (rnd.Float64()*2-1)*jitterDegrees
		lon := *donor.Longitude + (rnd.Float64()*2-1)*jitterDegrees
		city := "? Unbekannt"
		if donor.City != nil && *donor.City != "" {
			city = "? " + *donor.City
		}
		h.Latitude, h.Longitude, h.City = &lat, &lon, &city
		h.Country, h.CountryCode = donor.Country, donor.CountryCode
		h.GeoIPInterpolated = true
	}
}

func findDonor(hops []model.TraceHop, i int) *model.TraceHop {
	subnet := subnet24(*hops[i].IPAddress)
	at := func(j int) *model.TraceHop {
		if j < 0 || j >= len(hops) || !hops[j].HasLocation() {
			return nil
		}
		return &hops[j]
	}
	if subnet != "" {
		for off := 1; off <= sameSubnetRange; off++ {
			for _, j := range []int{i - off, i + off} {
				if d := at(j); d != nil && d.IPAddress != nil && subnet24(*d.IPAddress) == subnet {
					return d
				}
			}
		}
	}
	for off := 1; off < len(hops); off++ {
		for _, j := range []int{i - off, i + off} {
			if d := at(j); d != nil {
				return d
			}
		}
	}
	return nil
}

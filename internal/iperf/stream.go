package iperf

import (
	"encoding/json"
	"math"

	"iperf3-tracker/internal/model"
)

// Ausgabe von `iperf3 --json-stream` (ab iperf 3.17): eine JSON-Zeile je
// Ereignis ("start", "interval", "end", "error").
//
// Richtungen aus Sicht des Clients: Ohne -R sendet der Client (Upload), mit -R
// sendet der Server (Download). Bei --bidir beschreiben sum/sum_sent/
// sum_received den Upload und die *_bidir_reverse-Felder den Download.

type event struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// summary ist eine Summenzeile über alle Streams einer Richtung.
type summary struct {
	Bytes         int64    `json:"bytes"`
	BitsPerSecond float64  `json:"bits_per_second"`
	Retransmits   *int64   `json:"retransmits"` // nur TCP, nur beim Sender und nicht auf allen Plattformen
	JitterMs      *float64 `json:"jitter_ms"`   // nur UDP
	LostPercent   *float64 `json:"lost_percent"`
}

type intervalData struct {
	Sum             *summary `json:"sum"`
	SumBidirReverse *summary `json:"sum_bidir_reverse"`
}

type endData struct {
	SumSent                 *summary `json:"sum_sent"`
	SumReceived             *summary `json:"sum_received"`
	SumSentBidirReverse     *summary `json:"sum_sent_bidir_reverse"`
	SumReceivedBidirReverse *summary `json:"sum_received_bidir_reverse"`
	CPU                     *struct {
		HostTotal float64 `json:"host_total"`
	} `json:"cpu_utilization_percent"`
}

// Rates sind die aktuellen Bitraten eines Intervalls in Mbit/s; nil = keine Angabe.
type Rates struct {
	DownloadMbps *float64
	UploadMbps   *float64
}

// intervalRates ordnet ein Intervall-Ereignis Download und Upload zu.
func intervalRates(d intervalData, dir model.Direction) Rates {
	var r Rates
	switch dir {
	case model.DirectionDownload:
		r.DownloadMbps = mbps(d.Sum)
	case model.DirectionUpload:
		r.UploadMbps = mbps(d.Sum)
	case model.DirectionBidirectional:
		r.UploadMbps = mbps(d.Sum)
		r.DownloadMbps = mbps(d.SumBidirReverse)
	}
	return r
}

// endResult wandelt das Abschluss-Ereignis in Messwerte um. ok ist false,
// wenn für die geforderte Richtung keine Summen vorliegen.
func endResult(d endData, dir model.Direction) (r model.TestResult, ok bool) {
	var retrans []*int64
	switch dir {
	case model.DirectionDownload:
		ok = setFlow(&r.DownloadBandwidthMbps, &r.DownloadBytes, &r.DownloadJitterMs, &r.DownloadPacketLossPercent, d.SumSent, d.SumReceived)
		retrans = append(retrans, retransmits(d.SumSent))
	case model.DirectionUpload:
		ok = setFlow(&r.UploadBandwidthMbps, &r.UploadBytes, &r.UploadJitterMs, &r.UploadPacketLossPercent, d.SumSent, d.SumReceived)
		retrans = append(retrans, retransmits(d.SumSent))
	case model.DirectionBidirectional:
		up := setFlow(&r.UploadBandwidthMbps, &r.UploadBytes, &r.UploadJitterMs, &r.UploadPacketLossPercent, d.SumSent, d.SumReceived)
		down := setFlow(&r.DownloadBandwidthMbps, &r.DownloadBytes, &r.DownloadJitterMs, &r.DownloadPacketLossPercent, d.SumSentBidirReverse, d.SumReceivedBidirReverse)
		ok = up && down
		retrans = append(retrans, retransmits(d.SumSent), retransmits(d.SumSentBidirReverse))
	}
	for _, n := range retrans {
		if n != nil {
			total := *n
			if r.Retransmits != nil {
				total += *r.Retransmits
			}
			r.Retransmits = &total
		}
	}
	if d.CPU != nil {
		cpu := round2(d.CPU.HostTotal)
		r.CPUPercent = &cpu
	}
	return r, ok
}

// setFlow übernimmt die Werte einer Richtung. Maßgeblich ist die
// Empfängerseite, also das tatsächlich übertragene Volumen.
func setFlow(bw **float64, bytes **int64, jitter, loss **float64, sent, received *summary) bool {
	s := received
	if s == nil {
		s = sent
	}
	if s == nil {
		return false
	}
	*bw = mbps(s)
	b := s.Bytes
	*bytes = &b
	if s.JitterMs != nil {
		j := round2(*s.JitterMs)
		*jitter = &j
	}
	if s.LostPercent != nil {
		l := round2(*s.LostPercent)
		*loss = &l
	}
	return true
}

func retransmits(s *summary) *int64 {
	if s == nil {
		return nil
	}
	return s.Retransmits
}

func mbps(s *summary) *float64 {
	if s == nil {
		return nil
	}
	v := round2(s.BitsPerSecond / 1e6)
	return &v
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

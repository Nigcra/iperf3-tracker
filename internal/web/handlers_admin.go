package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/model"
)

// cleanupBefore liest days (≥ 0) und all. Ohne days und ohne all ist das
// Ergebnis eine Fehlermeldung, damit nicht versehentlich alles gelöscht wird.
func cleanupBefore(r *http.Request) (before *time.Time, all bool, err error) {
	days, err := queryInt64(r, "days")
	if err != nil {
		return nil, false, err
	}
	if days != nil {
		if *days < 0 {
			return nil, false, fmt.Errorf("Parameter \"days\" darf nicht negativ sein")
		}
		t := time.Now().AddDate(0, 0, -int(*days))
		before = &t
	}
	allParam, err := queryBool(r, "all")
	if err != nil {
		return nil, false, err
	}
	return before, allParam != nil && *allParam, nil
}

func (s *Server) handleCleanupTests(w http.ResponseWriter, r *http.Request, _ *model.User) {
	before, all, err := cleanupBefore(r)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	serverID, err := queryInt64(r, "server_id")
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if !all && before == nil && serverID == nil {
		writeError(w, http.StatusBadRequest, "Bitte days, server_id oder all=true angeben")
		return
	}
	n, err := s.store.DeleteTests(r.Context(), before, serverID)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message":       fmt.Sprintf("%d Test(s) gelöscht", n),
		"deleted_count": n,
	})
}

func (s *Server) handleCleanupTraces(w http.ResponseWriter, r *http.Request, _ *model.User) {
	before, all, err := cleanupBefore(r)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if !all && before == nil {
		writeError(w, http.StatusBadRequest, "Bitte days oder all=true angeben")
		return
	}
	if all {
		before = nil
	}
	traces, hops, err := s.store.DeleteTraces(r.Context(), before)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message":        fmt.Sprintf("%d Trace(s) und %d Hop(s) gelöscht", traces, hops),
		"deleted_traces": traces,
		"deleted_hops":   hops,
	})
}

func (s *Server) handleDatabaseStats(w http.ResponseWriter, r *http.Request, _ *model.User) {
	st, err := s.store.DatabaseStats(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// publicServers ist eine kuratierte Liste öffentlicher iperf3-Server; ihre
// Erreichbarkeit ist nicht garantiert.
var publicServers = []model.PublicServer{
	{Name: "Bouygues Telecom Paris", Host: "iperf.par2.as5410.net", Port: 5200, Location: "Paris, Frankreich", Provider: "Bouygues Telecom", Description: "Öffentlicher iperf3-Server in Paris"},
	{Name: "OVH Proof", Host: "proof.ovh.net", Port: 5201, Location: "Frankreich", Provider: "OVH", Description: "Öffentlicher Testserver von OVH"},
	{Name: "Online.net Paris", Host: "ping.online.net", Port: 5200, Location: "Paris, Frankreich", Provider: "Online.net", Description: "Öffentlicher iperf3-Server von Online.net"},
	{Name: "wilhelm.tel Hamburg", Host: "speedtest.wtnet.de", Port: 5200, Location: "Hamburg, Deutschland", Provider: "wilhelm.tel", Description: "Öffentlicher iperf3-Server von wilhelm.tel"},
	{Name: "Speedtest Frankfurt", Host: "speedtest.fra.de.as9136.net", Port: 5200, Location: "Frankfurt, Deutschland", Provider: "AS9136", Description: "Öffentlicher iperf3-Server in Frankfurt"},
	{Name: "Wobcom Wolfsburg", Host: "a400.speedtest.wobcom.de", Port: 5201, Location: "Wolfsburg, Deutschland", Provider: "WOBCOM", Description: "Öffentlicher iperf3-Server von WOBCOM (AS9136)"},
	{Name: "Uztelecom Taschkent", Host: "speedtest.uztelecom.uz", Port: 5200, Location: "Taschkent, Usbekistan", Provider: "Uztelecom", Description: "Öffentlicher iperf3-Server von Uztelecom"},
	{Name: "AT&T Ashburn VA", Host: "speedtest.ashb.va.us.as7018.net", Port: 5201, Location: "Ashburn, Virginia, USA", Provider: "AT&T", Description: "Öffentlicher iperf3-Server von AT&T in Virginia"},
	{Name: "Hurricane Electric", Host: "speedtest.dal.tx.us.he.net", Port: 5201, Location: "Dallas, Texas, USA", Provider: "Hurricane Electric", Description: "Öffentlicher Server von Hurricane Electric in Dallas"},
	{Name: "Vodafone Portugal", Host: "speedtest.vodafone.pt", Port: 5201, Location: "Portugal", Provider: "Vodafone", Description: "Öffentlicher iperf3-Server von Vodafone Portugal"},
	{Name: "Init7 Schweiz", Host: "speedtest.init7.net", Port: 5201, Location: "Schweiz", Provider: "Init7", Description: "Öffentlicher iperf3-Server von Init7"},
}

func (s *Server) handlePublicServers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, publicServers)
}

// handleSearchPublicServers sucht ohne Beachtung der Groß-/Kleinschreibung in
// Name, Ort und Anbieter.
func (s *Server) handleSearchPublicServers(w http.ResponseWriter, r *http.Request) {
	q, ok := r.URL.Query()["query"]
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "Parameter \"query\" fehlt")
		return
	}
	needle := strings.ToLower(q[0])
	results := []model.PublicServer{}
	for _, ps := range publicServers {
		if strings.Contains(strings.ToLower(ps.Name), needle) ||
			strings.Contains(strings.ToLower(ps.Location), needle) ||
			strings.Contains(strings.ToLower(ps.Provider), needle) {
			results = append(results, ps)
		}
	}
	writeJSON(w, http.StatusOK, results)
}

// handleInstallIperf startet nach Bestätigung in der Oberfläche die
// Installation von iperf3 über den Paketmanager.
func (s *Server) handleInstallIperf(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch err := s.runner.InstallAsync(); {
	case errors.Is(err, iperf.ErrInstallRunning):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, iperf.ErrNotInstallable):
		st := s.runner.Status()
		msg := err.Error()
		if st.Error != "" {
			msg += ": " + st.Error
		}
		writeError(w, http.StatusConflict, msg)
	case err != nil:
		writeInternal(w, r, err)
	default:
		slog.Info("iperf3-Installation angefordert", "benutzer", u.Username)
		writeJSON(w, http.StatusAccepted, s.runner.Status())
	}
}

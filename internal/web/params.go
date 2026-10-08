package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// pathID liest einen numerischen Pfadparameter; bei Fehlern wird mit 422
// geantwortet und ok ist false.
func pathID(w http.ResponseWriter, r *http.Request, name string) (id int64, ok bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("Ungültiger Parameter %q", name))
		return 0, false
	}
	return id, true
}

// queryInt liest einen optionalen Ganzzahl-Parameter im Bereich [lo, hi].
func queryInt(r *http.Request, name string, def, lo, hi int) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("Parameter %q muss eine Zahl zwischen %d und %d sein", name, lo, hi)
	}
	return n, nil
}

// queryBool liest einen optionalen Wahrheitswert; nil bedeutet „nicht angegeben".
// Akzeptiert wie FastAPI true/false, 1/0, yes/no, on/off.
func queryBool(r *http.Request, name string) (*bool, error) {
	v := strings.ToLower(r.URL.Query().Get(name))
	var b bool
	switch v {
	case "":
		return nil, nil
	case "true", "1", "yes", "on":
		b = true
	case "false", "0", "no", "off":
		b = false
	default:
		return nil, fmt.Errorf("Parameter %q muss true oder false sein", name)
	}
	return &b, nil
}

// queryInt64 liest einen optionalen Ganzzahl-Parameter; nil = nicht angegeben.
func queryInt64(r *http.Request, name string) (*int64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Parameter %q muss eine Zahl sein", name)
	}
	return &n, nil
}

// queryTime liest einen optionalen Zeitpunkt (ISO 8601, mit oder ohne
// Zeitzone, oder nur Datum). Angaben ohne Zeitzone gelten als UTC.
func queryTime(r *http.Request, name string) (*time.Time, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			t = t.UTC()
			return &t, nil
		}
	}
	return nil, fmt.Errorf("Parameter %q ist kein gültiger Zeitpunkt", name)
}

// paging liest skip (≥ 0) und limit (1–maxLimit, Standard defLimit).
func paging(r *http.Request, defLimit, maxLimit int) (skip, limit int, err error) {
	if skip, err = queryInt(r, "skip", 0, 0, 1<<31-1); err != nil {
		return 0, 0, err
	}
	limit, err = queryInt(r, "limit", defLimit, 1, maxLimit)
	return skip, limit, err
}

package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// paging liest skip (≥ 0) und limit (1–maxLimit, Standard defLimit).
func paging(r *http.Request, defLimit, maxLimit int) (skip, limit int, err error) {
	if skip, err = queryInt(r, "skip", 0, 0, 1<<31-1); err != nil {
		return 0, 0, err
	}
	limit, err = queryInt(r, "limit", defLimit, 1, maxLimit)
	return skip, limit, err
}

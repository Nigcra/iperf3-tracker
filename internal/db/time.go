package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Zeitstempel werden als TEXT in UTC mit fester Mikrosekunden-Genauigkeit
// gespeichert. Das feste Format ist lexikografisch sortierbar, sodass
// Datumsfilter direkt per Stringvergleich in SQL funktionieren.
const timeLayout = "2006-01-02T15:04:05.000000Z"

// Now liefert die aktuelle Zeit in UTC, gekürzt auf die gespeicherte
// Genauigkeit, damit API-Antworten vor und nach dem Speichern übereinstimmen.
func Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// FormatTime wandelt t in das Speicherformat um.
func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// FormatNullTime wandelt t in das Speicherformat um; nil ergibt NULL.
func FormatNullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return FormatTime(*t)
}

// Time liefert einen sql.Scanner für eine NOT-NULL-Zeitspalte.
func Time(dst *time.Time) sql.Scanner {
	return scanner(func(t *time.Time) {
		if t != nil {
			*dst = *t
		}
	})
}

// NullTime liefert einen sql.Scanner für eine nullbare Zeitspalte; NULL ergibt nil.
func NullTime(dst **time.Time) sql.Scanner {
	return scanner(func(t *time.Time) { *dst = t })
}

type scanner func(*time.Time)

func (set scanner) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		set(nil)
		return nil
	case time.Time:
		t := v.UTC()
		set(&t)
		return nil
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("Zeitstempel: unerwarteter Typ %T", src)
	}
	t, err := parseTime(s)
	if err != nil {
		return err
	}
	set(&t)
	return nil
}

// parseTime akzeptiert neben dem eigenen Format auch SQLites CURRENT_TIMESTAMP.
func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{timeLayout, time.RFC3339Nano, "2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("Zeitstempel %q nicht lesbar", s)
}

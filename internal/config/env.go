package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// EnvPrefix ist das Präfix aller Umgebungsvariablen.
const EnvPrefix = "IPERF3_"

// legacyEnv ordnet die früheren Variablennamen ohne Präfix ihrem Schlüssel
// zu. Sie gelten weiter, erzeugen aber eine Warnung im Log. Ist auch der neue
// Name gesetzt, gewinnt dieser.
var legacyEnv = []struct{ name, key string }{
	{"LISTEN_ADDR", "web.listen"},
	{"DB_PATH", "storage.path"},
	{"SECRET_KEY", "auth.secret_key"},
	{"SCHEDULER_ENABLED", "scheduler.enabled"},
	{"IPERF3_PATH", "iperf.path"},
	{"IPERF3_AUTO_INSTALL", "iperf.auto_install"},
	{"GEOIP_PATH", "geoip.path"},
	{"LOG_LEVEL", "log.level"},
}

// EnvName liefert den Variablennamen zu einem Schlüssel wie "web.listen".
func EnvName(key string) string {
	return EnvPrefix + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
}

// envField ist ein per Umgebung setzbarer Konfigurationswert.
type envField struct {
	key string // "abschnitt.schlüssel"
	val reflect.Value
}

// envFields listet alle Schlüssel der Konfiguration in Dateireihenfolge.
func (c *Config) envFields() []envField {
	var out []envField
	root := reflect.ValueOf(c).Elem()
	for i := 0; i < root.NumField(); i++ {
		section, ok := yamlName(root.Type().Field(i))
		sv := root.Field(i)
		if !ok || sv.Kind() != reflect.Struct {
			continue
		}
		for j := 0; j < sv.NumField(); j++ {
			name, ok := yamlName(sv.Type().Field(j))
			if !ok {
				continue
			}
			out = append(out, envField{key: section + "." + name, val: sv.Field(j)})
		}
	}
	return out
}

func yamlName(f reflect.StructField) (string, bool) {
	tag, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
	if tag == "" || tag == "-" {
		return "", false
	}
	return tag, true
}

// EnvKeys liefert alle Variablennamen (für Doku und Tests).
func EnvKeys() []string {
	var c Config
	var out []string
	for _, f := range c.envFields() {
		out = append(out, EnvName(f.key))
	}
	return out
}

// applyEnv überschreibt Werte aus der Umgebung: zuerst die veralteten
// Aliase, danach die neuen Namen, die damit Vorrang haben.
func (c *Config) applyEnv(lookup func(string) (string, bool)) error {
	fields := map[string]envField{}
	for _, f := range c.envFields() {
		fields[f.key] = f
	}
	for _, l := range legacyEnv {
		v, ok := lookup(l.name)
		if !ok || v == "" {
			continue
		}
		newName := EnvName(l.key)
		if nv, set := lookup(newName); set && nv != "" {
			c.Warnings = append(c.Warnings, fmt.Sprintf("Veraltete Umgebungsvariable %s wird ignoriert, da %s gesetzt ist", l.name, newName))
			continue
		}
		if err := setField(fields[l.key].val, v); err != nil {
			return fmt.Errorf("%s ist ungültig: %q", l.name, v)
		}
		c.Warnings = append(c.Warnings, fmt.Sprintf("Veraltete Umgebungsvariable %s – bitte %s verwenden", l.name, newName))
	}
	for _, f := range c.envFields() {
		name := EnvName(f.key)
		v, ok := lookup(name)
		if !ok || v == "" {
			continue
		}
		if err := setField(f.val, v); err != nil {
			return fmt.Errorf("%s ist ungültig: %q", name, v)
		}
	}
	return nil
}

func setField(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(n)
	default:
		return fmt.Errorf("Typ %s nicht unterstützt", v.Kind())
	}
	return nil
}

package handler

import "testing"

func TestMissingRequiredClashField(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]interface{}
		want string
	}{
		{"normal port", map[string]interface{}{"name": "n", "type": "ss", "server": "s", "port": 443}, ""},
		{"mieru port-range", map[string]interface{}{"name": "n", "type": "mieru", "server": "s", "port-range": "2090-2099"}, ""},
		{"hysteria2 ports", map[string]interface{}{"name": "n", "type": "hysteria2", "server": "s", "ports": "1000-2000"}, ""},
		{"no port at all", map[string]interface{}{"name": "n", "type": "ss", "server": "s"}, "port"},
		{"missing server", map[string]interface{}{"name": "n", "type": "ss", "port": 443}, "server"},
		{"missing name", map[string]interface{}{"type": "ss", "server": "s", "port": 443}, "name"},
	}
	for _, c := range cases {
		if got := missingRequiredClashField(c.cfg); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

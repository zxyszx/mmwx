package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const dialerProxyFixture = `proxies:
  - name: 落地
    type: ss
    server: 1.1.1.1
    port: 443
    cipher: aes-128-gcm
    password: old
    dialer-proxy: 中转组
  - name: 中转
    type: ss
    server: 2.2.2.2
    port: 443
    cipher: aes-128-gcm
    password: p
proxy-groups:
  - name: 中转组
    type: select
    proxies:
      - 中转
  - name: 落地组
    type: select
    proxies:
      - 落地
rules:
  - MATCH,落地组
`

func readDialerProxy(t *testing.T, path, proxyName string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, p := range cfg.Proxies {
		if p["name"] == proxyName {
			v, _ := p["dialer-proxy"].(string)
			return v
		}
	}
	t.Fatalf("proxy %q not found in:\n%s", proxyName, data)
	return ""
}

// 同步节点配置（数据库配置不含 dialer-proxy）时，不应冲掉订阅文件中的中转配置（issue #123）
func TestSyncNodesKeepsDialerProxy(t *testing.T) {
	newConfig := `{"name":"%s","type":"ss","server":"1.1.1.1","port":443,"cipher":"aes-128-gcm","password":"new"}`

	cases := []struct {
		name    string
		newName string
		sync    func(dir, newName string) error
	}{
		{"batch same name", "落地", func(dir, newName string) error {
			return batchSyncNodesToYAMLFiles(dir, []NodeUpdate{{OldName: "落地", NewName: newName, ClashConfigJSON: strings.Replace(newConfig, "%s", newName, 1)}})
		}},
		{"batch renamed", "落地2", func(dir, newName string) error {
			return batchSyncNodesToYAMLFiles(dir, []NodeUpdate{{OldName: "落地", NewName: newName, ClashConfigJSON: strings.Replace(newConfig, "%s", newName, 1)}})
		}},
		{"single same name", "落地", func(dir, newName string) error {
			return syncNodeToYAMLFiles(dir, "落地", newName, strings.Replace(newConfig, "%s", newName, 1))
		}},
		{"single renamed", "落地2", func(dir, newName string) error {
			return syncNodeToYAMLFiles(dir, "落地", newName, strings.Replace(newConfig, "%s", newName, 1))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sub.yaml")
			if err := os.WriteFile(path, []byte(dialerProxyFixture), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := tc.sync(dir, tc.newName); err != nil {
				t.Fatal(err)
			}
			if got := readDialerProxy(t, path, tc.newName); got != "中转组" {
				t.Fatalf("dialer-proxy = %q, want %q", got, "中转组")
			}
		})
	}
}

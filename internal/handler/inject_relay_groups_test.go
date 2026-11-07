package handler

import (
	"context"
	"path/filepath"
	"testing"

	"miaomiaowu/internal/storage"

	"gopkg.in/yaml.v3"
)

func relayTestRepo(t *testing.T) *storage.TrafficRepository {
	t.Helper()
	repo, err := storage.NewTrafficRepository(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewTrafficRepository: %v", err)
	}
	return repo
}

func mustNode(t *testing.T, repo *storage.TrafficRepository, n storage.Node) storage.Node {
	t.Helper()
	created, err := repo.CreateNode(context.Background(), n)
	if err != nil {
		t.Fatalf("CreateNode(%s): %v", n.NodeName, err)
	}
	return created
}

// rootMappingFromYAML parses YAML and returns the top-level mapping node
// (the shape injectRelayGroups expects).
func rootMappingFromYAML(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("unmarshal yaml: %v", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		t.Fatal("unexpected yaml shape")
	}
	return root.Content[0]
}

func proxyNamesOf(rootMap *yaml.Node) map[string]bool {
	names := make(map[string]bool)
	for i := 0; i < len(rootMap.Content); i += 2 {
		if rootMap.Content[i].Value != "proxies" {
			continue
		}
		for _, pn := range rootMap.Content[i+1].Content {
			if pn.Kind == yaml.MappingNode {
				names[yamlMapGet(pn, "name")] = true
			}
		}
	}
	return names
}

func findGroupProxies(rootMap *yaml.Node, groupName string) ([]string, bool) {
	for i := 0; i < len(rootMap.Content); i += 2 {
		if rootMap.Content[i].Value != "proxy-groups" {
			continue
		}
		for _, g := range rootMap.Content[i+1].Content {
			if g.Kind != yaml.MappingNode || yamlMapGet(g, "name") != groupName {
				continue
			}
			for j := 0; j < len(g.Content); j += 2 {
				if g.Content[j].Value == "proxies" {
					var out []string
					for _, p := range g.Content[j+1].Content {
						out = append(out, p.Value)
					}
					return out, true
				}
			}
		}
	}
	return nil, false
}

// TestInjectRelayGroups_BackfillsAndDropsDisabled verifies that for an existing
// landing (source) node, missing enabled relay members are back-filled into the
// file's proxies, disabled members are dropped, and the relay group + dialer-proxy
// are injected.
func TestInjectRelayGroups_BackfillsAndDropsDisabled(t *testing.T) {
	repo := relayTestRepo(t)
	const user = "admin"

	memberA := mustNode(t, repo, storage.Node{
		Username: user, NodeName: "Member-A", Protocol: "trojan",
		ClashConfig: `{"name":"Member-A","type":"trojan","server":"a.example.com","port":443}`,
		Enabled:     true,
	})
	memberC := mustNode(t, repo, storage.Node{
		Username: user, NodeName: "Member-Disabled", Protocol: "trojan",
		ClashConfig: `{"name":"Member-Disabled","type":"trojan","server":"c.example.com","port":443}`,
		Enabled:     false,
	})
	mustNode(t, repo, storage.Node{
		Username: user, NodeName: "Source", Protocol: "vmess",
		ClashConfig:       `{"name":"Source","type":"vmess","server":"s.example.com","port":443}`,
		Enabled:           true,
		RelayGroupName:    "中转HK",
		RelayGroupNodeIDs: []int64{memberA.ID, memberC.ID},
	})

	rootMap := rootMappingFromYAML(t, `
proxies:
  - name: Source
    type: vmess
    server: s.example.com
    port: 443
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - Source
`)

	injectRelayGroups(context.Background(), repo, user, rootMap)

	names := proxyNamesOf(rootMap)
	if !names["Member-A"] {
		t.Error("enabled member Member-A should be back-filled into root proxies")
	}
	if names["Member-Disabled"] {
		t.Error("disabled member should NOT be back-filled into root proxies")
	}

	members, ok := findGroupProxies(rootMap, "中转HK")
	if !ok {
		t.Fatal("relay group 中转HK not injected into proxy-groups")
	}
	if len(members) != 1 || members[0] != "Member-A" {
		t.Errorf("relay group members = %v, want [Member-A]", members)
	}

	// Source must carry dialer-proxy pointing at the relay group.
	for i := 0; i < len(rootMap.Content); i += 2 {
		if rootMap.Content[i].Value != "proxies" {
			continue
		}
		for _, pn := range rootMap.Content[i+1].Content {
			if yamlMapGet(pn, "name") == "Source" {
				if dp := yamlMapGet(pn, "dialer-proxy"); dp != "中转HK" {
					t.Errorf("Source dialer-proxy = %q, want %q", dp, "中转HK")
				}
			}
		}
	}
}

// TestInjectRelayGroups_SourceNotInSubscription verifies that when the landing
// (source) node is absent from the subscription, no relay group is inserted.
func TestInjectRelayGroups_SourceNotInSubscription(t *testing.T) {
	repo := relayTestRepo(t)
	const user = "admin"

	memberA := mustNode(t, repo, storage.Node{
		Username: user, NodeName: "Member-A", Protocol: "trojan",
		ClashConfig: `{"name":"Member-A","type":"trojan","server":"a.example.com","port":443}`,
		Enabled:     true,
	})
	mustNode(t, repo, storage.Node{
		Username: user, NodeName: "Source", Protocol: "vmess",
		ClashConfig:       `{"name":"Source","type":"vmess","server":"s.example.com","port":443}`,
		Enabled:           true,
		RelayGroupName:    "中转HK",
		RelayGroupNodeIDs: []int64{memberA.ID},
	})

	// Subscription does not contain the source node.
	rootMap := rootMappingFromYAML(t, `
proxies:
  - name: Other
    type: ss
    server: o.example.com
    port: 8388
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - Other
`)

	injectRelayGroups(context.Background(), repo, user, rootMap)

	if _, ok := findGroupProxies(rootMap, "中转HK"); ok {
		t.Error("relay group should NOT be inserted when source node is absent from subscription")
	}
	if proxyNamesOf(rootMap)["Member-A"] {
		t.Error("member should NOT be back-filled when source node is absent")
	}
}

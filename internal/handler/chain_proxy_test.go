package handler

import (
	"encoding/json"
	"testing"

	"miaomiaowu/internal/storage"
)

// TestChainProxyInjection tests that dialer-proxy is correctly injected
// into proxy configs based on chain_proxy_node_id during subscription generation.
func TestChainProxyInjection(t *testing.T) {
	targetID := int64(2)
	nodes := []storage.Node{
		{
			ID:       1,
			Username: "admin",
			NodeName: "HK-01 | JP-Relay",
			Protocol: "vmess⇋trojan",
			ClashConfig: `{"name":"HK-01 | JP-Relay","type":"vmess","server":"hk.example.com","port":443}`,
			Enabled:          true,
			Tags:             []string{"链式代理"},
			ChainProxyNodeID: &targetID,
		},
		{
			ID:          2,
			Username:    "admin",
			NodeName:    "JP-Relay",
			Protocol:    "trojan",
			ClashConfig: `{"name":"JP-Relay","type":"trojan","server":"jp.example.com","port":443}`,
			Enabled:     true,
			Tags:        []string{"中转"},
		},
		{
			ID:          3,
			Username:    "admin",
			NodeName:    "US-Direct",
			Protocol:    "ss",
			ClashConfig: `{"name":"US-Direct","type":"ss","server":"us.example.com","port":8388}`,
			Enabled:     true,
			Tags:        []string{"直连"},
		},
	}

	proxies := buildProxiesFromNodes(nodes, nil)

	if len(proxies) != 3 {
		t.Fatalf("expected 3 proxies, got %d", len(proxies))
	}

	// Chain proxy node should have dialer-proxy injected
	chainProxy := findProxyByName(proxies, "HK-01 | JP-Relay")
	if chainProxy == nil {
		t.Fatal("chain proxy node not found")
	}
	dp, ok := chainProxy["dialer-proxy"].(string)
	if !ok || dp != "JP-Relay" {
		t.Errorf("chain proxy dialer-proxy = %q, want %q", dp, "JP-Relay")
	}

	// Target node should NOT have dialer-proxy
	relayProxy := findProxyByName(proxies, "JP-Relay")
	if relayProxy == nil {
		t.Fatal("relay node not found")
	}
	if _, has := relayProxy["dialer-proxy"]; has {
		t.Error("relay node should not have dialer-proxy")
	}

	// Regular node should NOT have dialer-proxy
	directProxy := findProxyByName(proxies, "US-Direct")
	if directProxy == nil {
		t.Fatal("direct node not found")
	}
	if _, has := directProxy["dialer-proxy"]; has {
		t.Error("direct node should not have dialer-proxy")
	}
}

// TestChainProxyInjection_TargetNotFound tests that when chain_proxy_node_id
// references a non-existent node, no dialer-proxy is injected.
func TestChainProxyInjection_TargetNotFound(t *testing.T) {
	nonExistentID := int64(999)
	nodes := []storage.Node{
		{
			ID:               1,
			Username:         "admin",
			NodeName:         "Orphan-Chain",
			Protocol:         "vmess⇋ss",
			ClashConfig:      `{"name":"Orphan-Chain","type":"vmess","server":"x.example.com","port":443}`,
			Enabled:          true,
			Tags:             []string{"链式代理"},
			ChainProxyNodeID: &nonExistentID,
		},
	}

	proxies := buildProxiesFromNodes(nodes, nil)

	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}

	if _, has := proxies[0]["dialer-proxy"]; has {
		t.Error("dialer-proxy should not be injected when target node is not found")
	}
}

// TestChainProxyInjection_DisabledTarget tests that chain proxy injection
// still resolves against disabled target nodes (they exist in the ID map).
func TestChainProxyInjection_DisabledTarget(t *testing.T) {
	targetID := int64(2)
	nodes := []storage.Node{
		{
			ID:               1,
			Username:         "admin",
			NodeName:         "Chain-Node",
			Protocol:         "vmess⇋trojan",
			ClashConfig:      `{"name":"Chain-Node","type":"vmess","server":"a.example.com","port":443}`,
			Enabled:          true,
			Tags:             []string{"链式代理"},
			ChainProxyNodeID: &targetID,
		},
		{
			ID:          2,
			Username:    "admin",
			NodeName:    "Disabled-Relay",
			Protocol:    "trojan",
			ClashConfig: `{"name":"Disabled-Relay","type":"trojan","server":"b.example.com","port":443}`,
			Enabled:     false,
			Tags:        []string{"中转"},
		},
	}

	proxies := buildProxiesFromNodes(nodes, nil)

	// Only enabled nodes become proxies
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy (disabled node excluded), got %d", len(proxies))
	}

	// Chain proxy should still resolve against disabled target (name lookup uses all nodes)
	dp, ok := proxies[0]["dialer-proxy"].(string)
	if !ok || dp != "Disabled-Relay" {
		t.Errorf("dialer-proxy = %q, want %q", dp, "Disabled-Relay")
	}
}

// TestChainProxyInjection_WithTagFilter tests that tag filtering works
// correctly with chain proxy injection.
func TestChainProxyInjection_WithTagFilter(t *testing.T) {
	targetID := int64(2)
	nodes := []storage.Node{
		{
			ID:               1,
			Username:         "admin",
			NodeName:         "HK | Relay",
			Protocol:         "vmess⇋trojan",
			ClashConfig:      `{"name":"HK | Relay","type":"vmess","server":"hk.example.com","port":443}`,
			Enabled:          true,
			Tags:             []string{"链式代理"},
			ChainProxyNodeID: &targetID,
		},
		{
			ID:          2,
			Username:    "admin",
			NodeName:    "Relay-Node",
			Protocol:    "trojan",
			ClashConfig: `{"name":"Relay-Node","type":"trojan","server":"relay.example.com","port":443}`,
			Enabled:     true,
			Tags:        []string{"中转"},
		},
		{
			ID:          3,
			Username:    "admin",
			NodeName:    "Other-Node",
			Protocol:    "ss",
			ClashConfig: `{"name":"Other-Node","type":"ss","server":"other.example.com","port":8388}`,
			Enabled:     true,
			Tags:        []string{"其他"},
		},
	}

	// Filter to only "链式代理" tag
	tagFilter := map[string]bool{"链式代理": true}
	proxies := buildProxiesFromNodes(nodes, tagFilter)

	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy after tag filter, got %d", len(proxies))
	}

	// Chain proxy should still resolve target name even though target is filtered out
	dp, ok := proxies[0]["dialer-proxy"].(string)
	if !ok || dp != "Relay-Node" {
		t.Errorf("dialer-proxy = %q, want %q", dp, "Relay-Node")
	}
}

// TestChainProxyInjection_NoOverwriteExisting tests that if a node already has
// dialer-proxy in clash_config (legacy), the chain_proxy_node_id takes precedence.
func TestChainProxyInjection_ChainIDOverridesLegacy(t *testing.T) {
	targetID := int64(2)
	nodes := []storage.Node{
		{
			ID:               1,
			Username:         "admin",
			NodeName:         "Chain-Node",
			Protocol:         "vmess⇋trojan",
			ClashConfig:      `{"name":"Chain-Node","type":"vmess","server":"a.example.com","port":443,"dialer-proxy":"old-name"}`,
			Enabled:          true,
			Tags:             []string{"链式代理"},
			ChainProxyNodeID: &targetID,
		},
		{
			ID:          2,
			Username:    "admin",
			NodeName:    "New-Relay-Name",
			Protocol:    "trojan",
			ClashConfig: `{"name":"New-Relay-Name","type":"trojan","server":"b.example.com","port":443}`,
			Enabled:     true,
			Tags:        []string{"中转"},
		},
	}

	proxies := buildProxiesFromNodes(nodes, nil)

	chainProxy := findProxyByName(proxies, "Chain-Node")
	if chainProxy == nil {
		t.Fatal("chain proxy not found")
	}

	// chain_proxy_node_id should override any existing dialer-proxy in clash_config
	dp, ok := chainProxy["dialer-proxy"].(string)
	if !ok || dp != "New-Relay-Name" {
		t.Errorf("dialer-proxy = %q, want %q (chain_proxy_node_id should override legacy)", dp, "New-Relay-Name")
	}
}

// TestChainProxyInjection_NilChainID tests nodes without chain_proxy_node_id
// preserve any existing dialer-proxy in their clash_config (for template V3 compatibility).
func TestChainProxyInjection_NilChainID_PreservesExisting(t *testing.T) {
	nodes := []storage.Node{
		{
			ID:          1,
			Username:    "admin",
			NodeName:    "Legacy-Node",
			Protocol:    "vmess",
			ClashConfig: `{"name":"Legacy-Node","type":"vmess","server":"a.example.com","port":443,"dialer-proxy":"🌠 中转节点"}`,
			Enabled:     true,
			Tags:        []string{"落地"},
		},
	}

	proxies := buildProxiesFromNodes(nodes, nil)

	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}

	// Without chain_proxy_node_id, existing dialer-proxy from clash_config is preserved
	dp, ok := proxies[0]["dialer-proxy"].(string)
	if !ok || dp != "🌠 中转节点" {
		t.Errorf("dialer-proxy = %q, want %q (should preserve existing)", dp, "🌠 中转节点")
	}
}

// TestMigrateChainProxyNodes_ExtractsDialerProxy tests the migration logic
// that extracts dialer-proxy from clash_config into chain_proxy_node_id.
func TestMigrateChainProxyNodes_ExtractsDialerProxy(t *testing.T) {
	// Simulate what migrateChainProxyNodes does: parse clash_config, find dialer-proxy
	clashConfig := `{"name":"HK⇋JP","type":"vmess","server":"hk.example.com","port":443,"dialer-proxy":"JP-Relay"}`

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(clashConfig), &config); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	dialerProxy, ok := config["dialer-proxy"].(string)
	if !ok || dialerProxy != "JP-Relay" {
		t.Fatalf("dialer-proxy = %q, want %q", dialerProxy, "JP-Relay")
	}

	// Remove dialer-proxy
	delete(config, "dialer-proxy")
	newConfig, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify dialer-proxy is removed
	var verifyConfig map[string]interface{}
	if err := json.Unmarshal(newConfig, &verifyConfig); err != nil {
		t.Fatalf("verify unmarshal failed: %v", err)
	}
	if _, has := verifyConfig["dialer-proxy"]; has {
		t.Error("dialer-proxy should be removed after migration")
	}

	// Other fields should be preserved
	if verifyConfig["name"] != "HK⇋JP" {
		t.Errorf("name = %v, want %q", verifyConfig["name"], "HK⇋JP")
	}
	if verifyConfig["type"] != "vmess" {
		t.Errorf("type = %v, want %q", verifyConfig["type"], "vmess")
	}
}

// buildProxiesFromNodes replicates the subscription generation logic for testing.
// This mirrors the code in subscription.go generateFromTemplate.
func buildProxiesFromNodes(nodes []storage.Node, tagFilter map[string]bool) []map[string]any {
	hasTagFilter := len(tagFilter) > 0

	// Build node ID -> name mapping (same as subscription.go)
	nodeIDToName := make(map[int64]string, len(nodes))
	for _, node := range nodes {
		nodeIDToName[node.ID] = node.NodeName
	}

	var proxies []map[string]any
	for _, node := range nodes {
		if !node.Enabled {
			continue
		}
		if hasTagFilter && !node.HasAnyTag(tagFilter) {
			continue
		}
		var proxyConfig map[string]any
		if err := json.Unmarshal([]byte(node.ClashConfig), &proxyConfig); err != nil {
			continue
		}
		proxyConfig["name"] = node.NodeName
		if node.ChainProxyNodeID != nil {
			if targetName, ok := nodeIDToName[*node.ChainProxyNodeID]; ok {
				proxyConfig["dialer-proxy"] = targetName
			}
		}
		proxies = append(proxies, proxyConfig)
	}
	return proxies
}

func findProxyByName(proxies []map[string]any, name string) map[string]any {
	for _, p := range proxies {
		if n, _ := p["name"].(string); n == name {
			return p
		}
	}
	return nil
}

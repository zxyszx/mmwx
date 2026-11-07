package handler

import (
	"encoding/json"
	"testing"

	"miaomiaowu/internal/storage"
)

// buildRelayData replicates the relay-group portion of generateFromTemplate for
// testing: it returns the proxies that get written to the root `proxies` field
// (main proxies + back-filled relay members) and the generated relay proxy-groups.
// This mirrors the code in subscription.go generateFromTemplate.
func buildRelayData(nodes []storage.Node, tagFilter map[string]bool) (rootProxies []map[string]any, relayGroups []map[string]any) {
	hasTagFilter := len(tagFilter) > 0

	nodeIDToName := make(map[int64]string, len(nodes))
	nodeByID := make(map[int64]storage.Node, len(nodes))
	for _, node := range nodes {
		nodeIDToName[node.ID] = node.NodeName
		nodeByID[node.ID] = node
	}

	buildProxyConfig := func(node storage.Node) (map[string]any, bool) {
		var pc map[string]any
		if err := json.Unmarshal([]byte(node.ClashConfig), &pc); err != nil {
			return nil, false
		}
		pc["name"] = node.NodeName
		if node.ChainProxyNodeID != nil {
			if targetName, ok := nodeIDToName[*node.ChainProxyNodeID]; ok {
				pc["dialer-proxy"] = targetName
			}
		}
		if len(node.RelayGroupNodeIDs) > 0 && node.RelayGroupName != "" {
			pc["dialer-proxy"] = node.RelayGroupName
		}
		return pc, true
	}

	var proxies []map[string]any
	inRootProxies := make(map[string]bool)
	for _, node := range nodes {
		if !node.Enabled {
			continue
		}
		if hasTagFilter && !node.HasAnyTag(tagFilter) {
			continue
		}
		pc, ok := buildProxyConfig(node)
		if !ok {
			continue
		}
		proxies = append(proxies, pc)
		inRootProxies[node.NodeName] = true
	}

	relayGroupMap := make(map[string]map[string]any)
	var extraProxies []map[string]any
	for _, node := range nodes {
		if !node.Enabled || len(node.RelayGroupNodeIDs) == 0 || node.RelayGroupName == "" {
			continue
		}
		if hasTagFilter && !node.HasAnyTag(tagFilter) {
			continue
		}
		if _, exists := relayGroupMap[node.RelayGroupName]; exists {
			continue
		}
		var groupProxies []string
		for _, rid := range node.RelayGroupNodeIDs {
			member, ok := nodeByID[rid]
			if !ok || !member.Enabled {
				continue
			}
			groupProxies = append(groupProxies, member.NodeName)
			if !inRootProxies[member.NodeName] {
				if pc, ok := buildProxyConfig(member); ok {
					extraProxies = append(extraProxies, pc)
					inRootProxies[member.NodeName] = true
				}
			}
		}
		if len(groupProxies) > 0 {
			relayGroupMap[node.RelayGroupName] = map[string]any{
				"name":    node.RelayGroupName,
				"type":    "url-test",
				"proxies": groupProxies,
			}
		}
	}
	for _, rg := range relayGroupMap {
		relayGroups = append(relayGroups, rg)
	}

	rootProxies = make([]map[string]any, 0, len(proxies)+len(extraProxies))
	rootProxies = append(rootProxies, proxies...)
	rootProxies = append(rootProxies, extraProxies...)
	return rootProxies, relayGroups
}

// TestRelayGroup_UnderlyingNodeBackfilled is the core regression test: relay
// members that are filtered out by tag selection must still be back-filled into
// the root proxies so the relay proxy-group references resolve.
func TestRelayGroup_UnderlyingNodeBackfilled(t *testing.T) {
	nodes := []storage.Node{
		{
			ID:                1,
			NodeName:          "Source",
			ClashConfig:       `{"name":"Source","type":"vmess","server":"s.example.com","port":443}`,
			Enabled:           true,
			Tags:              []string{"个人"},
			RelayGroupName:    "中转HK",
			RelayGroupNodeIDs: []int64{2, 3},
		},
		{
			ID:          2,
			NodeName:    "Member-A",
			ClashConfig: `{"name":"Member-A","type":"trojan","server":"a.example.com","port":443}`,
			Enabled:     true,
			Tags:        []string{"中转"},
		},
		{
			ID:          3,
			NodeName:    "Member-B",
			ClashConfig: `{"name":"Member-B","type":"trojan","server":"b.example.com","port":443}`,
			Enabled:     true,
			Tags:        []string{"中转"},
		},
	}

	rootProxies, relayGroups := buildRelayData(nodes, map[string]bool{"个人": true})

	// Source + both members must all be in root proxies.
	for _, name := range []string{"Source", "Member-A", "Member-B"} {
		if findProxyByName(rootProxies, name) == nil {
			t.Errorf("root proxies missing %q", name)
		}
	}

	// Source carries dialer-proxy pointing at the relay group.
	src := findProxyByName(rootProxies, "Source")
	if dp, _ := src["dialer-proxy"].(string); dp != "中转HK" {
		t.Errorf("source dialer-proxy = %q, want %q", dp, "中转HK")
	}

	// Exactly one relay group with both members.
	if len(relayGroups) != 1 {
		t.Fatalf("expected 1 relay group, got %d", len(relayGroups))
	}
	members, _ := relayGroups[0]["proxies"].([]string)
	if len(members) != 2 {
		t.Errorf("relay group members = %v, want 2 entries", members)
	}
}

// TestRelayGroup_DisabledMemberDropped verifies disabled/missing members are
// dropped from the group and not back-filled (no dangling reference).
func TestRelayGroup_DisabledMemberDropped(t *testing.T) {
	nodes := []storage.Node{
		{
			ID:                1,
			NodeName:          "Source",
			ClashConfig:       `{"name":"Source","type":"vmess","server":"s.example.com","port":443}`,
			Enabled:           true,
			Tags:              []string{"个人"},
			RelayGroupName:    "中转HK",
			RelayGroupNodeIDs: []int64{2, 3, 999},
		},
		{
			ID:          2,
			NodeName:    "Member-A",
			ClashConfig: `{"name":"Member-A","type":"trojan","server":"a.example.com","port":443}`,
			Enabled:     true,
			Tags:        []string{"中转"},
		},
		{
			ID:          3,
			NodeName:    "Member-Disabled",
			ClashConfig: `{"name":"Member-Disabled","type":"trojan","server":"b.example.com","port":443}`,
			Enabled:     false,
			Tags:        []string{"中转"},
		},
	}

	rootProxies, relayGroups := buildRelayData(nodes, map[string]bool{"个人": true})

	if findProxyByName(rootProxies, "Member-Disabled") != nil {
		t.Error("disabled member should not be back-filled into root proxies")
	}
	if len(relayGroups) != 1 {
		t.Fatalf("expected 1 relay group, got %d", len(relayGroups))
	}
	members, _ := relayGroups[0]["proxies"].([]string)
	if len(members) != 1 || members[0] != "Member-A" {
		t.Errorf("relay group members = %v, want [Member-A] (disabled/missing dropped)", members)
	}
}

// TestRelayGroup_MemberAlreadyInProxies_NoDuplicate verifies a member that is
// already in the main proxies (passed the tag filter) is not duplicated.
func TestRelayGroup_MemberAlreadyInProxies_NoDuplicate(t *testing.T) {
	nodes := []storage.Node{
		{
			ID:                1,
			NodeName:          "Source",
			ClashConfig:       `{"name":"Source","type":"vmess","server":"s.example.com","port":443}`,
			Enabled:           true,
			Tags:              []string{"个人"},
			RelayGroupName:    "中转HK",
			RelayGroupNodeIDs: []int64{2},
		},
		{
			ID:          2,
			NodeName:    "Member-A",
			ClashConfig: `{"name":"Member-A","type":"trojan","server":"a.example.com","port":443}`,
			Enabled:     true,
			Tags:        []string{"个人"}, // also selected → already in main proxies
		},
	}

	rootProxies, _ := buildRelayData(nodes, map[string]bool{"个人": true})

	count := 0
	for _, p := range rootProxies {
		if n, _ := p["name"].(string); n == "Member-A" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Member-A appears %d times in root proxies, want 1", count)
	}
}

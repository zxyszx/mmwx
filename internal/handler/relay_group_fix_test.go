package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"miaomiaowu/internal/auth"
	"miaomiaowu/internal/storage"

	"gopkg.in/yaml.v3"
)

// TestNodeUpdate_PartialPayloadPreservesEnabled 回归:解除/创建中转组只发
// {relay_group_name, relay_group_node_ids}(不带 enabled),不应把节点误置为禁用。
func TestNodeUpdate_PartialPayloadPreservesEnabled(t *testing.T) {
	repo := relayTestRepo(t)
	const user = "admin"

	node := mustNode(t, repo, storage.Node{
		Username: user, NodeName: "Landing", Protocol: "vmess",
		ClashConfig: `{"name":"Landing","type":"vmess","server":"s.example.com","port":443}`,
		Enabled:     true,
	})

	h := NewNodesHandler(repo, t.TempDir())

	// 模拟前端"解除中转组":局部 payload,无 enabled
	body := strings.NewReader(`{"relay_group_name":"","relay_group_node_ids":[]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/nodes/"+int64ToString(node.ID), body)
	req = req.WithContext(auth.ContextWithUsername(context.Background(), user))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", rec.Code, rec.Body.String())
	}

	nodes, err := repo.ListNodes(context.Background(), user)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	var got *storage.Node
	for i := range nodes {
		if nodes[i].ID == node.ID {
			got = &nodes[i]
			break
		}
	}
	if got == nil {
		t.Fatal("node not found after update")
	}
	if !got.Enabled {
		t.Error("局部更新(无 enabled)后节点被误置为禁用 —— 应保持启用")
	}
	if got.RelayGroupName != "" {
		t.Errorf("relay_group_name 应被清空, got %q", got.RelayGroupName)
	}
}

// TestNodeUpdate_ExplicitDisableStillWorks 确保显式 enabled:false 仍生效(未被守卫误伤)。
func TestNodeUpdate_ExplicitDisableStillWorks(t *testing.T) {
	repo := relayTestRepo(t)
	const user = "admin"
	node := mustNode(t, repo, storage.Node{
		Username: user, NodeName: "N", Protocol: "vmess",
		ClashConfig: `{"name":"N","type":"vmess","server":"s.example.com","port":443}`,
		Enabled:     true,
	})

	h := NewNodesHandler(repo, t.TempDir())
	req := httptest.NewRequest(http.MethodPut, "/api/admin/nodes/"+int64ToString(node.ID),
		strings.NewReader(`{"enabled":false}`))
	req = req.WithContext(auth.ContextWithUsername(context.Background(), user))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	nodes, _ := repo.ListNodes(context.Background(), user)
	for _, n := range nodes {
		if n.ID == node.ID && n.Enabled {
			t.Error("显式 enabled:false 应使节点禁用")
		}
	}
}

// TestAnyToYAMLNode_StringSlice 回归:中转组 proxies 以 []string 传入时
// 应序列化为 YAML 序列,而非空字符串(否则中转组成员丢失)。
func TestAnyToYAMLNode_StringSlice(t *testing.T) {
	n := anyToYAMLNode([]string{"中转1", "中转2"})
	if n.Kind != yaml.SequenceNode {
		t.Fatalf("[]string 应转为 SequenceNode, got kind=%v value=%q", n.Kind, n.Value)
	}
	if len(n.Content) != 2 || n.Content[0].Value != "中转1" || n.Content[1].Value != "中转2" {
		t.Errorf("序列内容不正确: %+v", n.Content)
	}
}

// TestInjectRelayGroupsIntoTemplate_PopulatesMembers 端到端:模板路径注入的中转组
// 成员(来自 []string)不应为空。
func TestInjectRelayGroupsIntoTemplate_PopulatesMembers(t *testing.T) {
	tpl := "proxies: []\nproxy-groups:\n  - name: PROXY\n    type: select\n    proxies: [DIRECT]\n"
	relayGroups := []map[string]any{{
		"name":    "中转组",
		"type":    "url-test",
		"proxies": []string{"中转1", "中转2"},
	}}
	out, err := injectRelayGroupsIntoTemplate(tpl, relayGroups)
	if err != nil {
		t.Fatalf("injectRelayGroupsIntoTemplate: %v", err)
	}
	if strings.Contains(out, `proxies: ""`) {
		t.Errorf("中转组成员被序列化为空字符串:\n%s", out)
	}
	if !strings.Contains(out, "中转1") || !strings.Contains(out, "中转2") {
		t.Errorf("中转组成员缺失:\n%s", out)
	}
}

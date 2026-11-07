package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestNormalizeProxyFields_ContainerNilDropped 复现并锁定原始 bug：
// ws-opts 为 null 时曾被转成 "",导致 mihomo 报
// `proxy 91: 'ws-opts' expected a map, got 'string'`。
func TestNormalizeProxyFields_ContainerNilDropped(t *testing.T) {
	proxy := map[string]any{
		"name":         "节点A",
		"type":         "vmess",
		"server":       "a.example.com",
		"port":         443,
		"uuid":         "u-1",
		"network":      "ws",
		"ws-opts":      nil,
		"grpc-opts":    nil,
		"h2-opts":      nil,
		"reality-opts": nil,
	}

	normalizeProxyFields(proxy)

	for _, key := range []string{"ws-opts", "grpc-opts", "h2-opts", "reality-opts"} {
		if v, exists := proxy[key]; exists {
			t.Fatalf("容器字段 %q 为 nil 时应删除，实际保留为 %#v", key, v)
		}
	}
	if proxy["name"] != "节点A" || proxy["network"] != "ws" {
		t.Fatalf("正常字段被误改: %#v", proxy)
	}
}

// TestNormalizeProxyFields_ContainerEmptyStringDropped 覆盖自愈场景：
// 数据库里已经存下的坏值是字符串 "",而不是 null。
func TestNormalizeProxyFields_ContainerEmptyStringDropped(t *testing.T) {
	proxy := map[string]any{
		"name":        "节点B",
		"type":        "vless",
		"ws-opts":     "",
		"headers":     "",
		"alpn":        "",
		"plugin-opts": "",
	}

	normalizeProxyFields(proxy)

	for _, key := range []string{"ws-opts", "headers", "alpn", "plugin-opts"} {
		if v, exists := proxy[key]; exists {
			t.Fatalf("容器字段 %q 为空字符串时应删除（自愈已存坏数据），实际为 %#v", key, v)
		}
	}
}

// TestNormalizeProxyFields_ScalarNilBecomesEmptyString 确认 short-id 的历史修复没被破坏。
func TestNormalizeProxyFields_ScalarNilBecomesEmptyString(t *testing.T) {
	proxy := map[string]any{
		"name":     "节点C",
		"short-id": nil,
		"sni":      nil,
		"flow":     nil,
	}

	normalizeProxyFields(proxy)

	for _, key := range []string{"short-id", "sni", "flow"} {
		v, exists := proxy[key]
		if !exists {
			t.Fatalf("标量字段 %q 不应被删除", key)
		}
		if s, ok := v.(string); !ok || s != "" {
			t.Fatalf("标量字段 %q 应为空字符串，实际为 %#v", key, v)
		}
	}
}

// TestNormalizeProxyFields_ValidContainerPreserved 有效的容器值必须原样保留。
func TestNormalizeProxyFields_ValidContainerPreserved(t *testing.T) {
	proxy := map[string]any{
		"name": "节点D",
		"ws-opts": map[string]any{
			"path": "/ws",
			"headers": map[string]any{
				"Host": "d.example.com",
			},
		},
		"alpn": []any{"h2", "http/1.1"},
	}

	normalizeProxyFields(proxy)

	wsOpts, ok := proxy["ws-opts"].(map[string]any)
	if !ok {
		t.Fatalf("有效 ws-opts 应保持为 map，实际为 %T", proxy["ws-opts"])
	}
	if wsOpts["path"] != "/ws" {
		t.Fatalf("ws-opts.path 被改动: %#v", wsOpts)
	}
	headers, ok := wsOpts["headers"].(map[string]any)
	if !ok || headers["Host"] != "d.example.com" {
		t.Fatalf("嵌套 headers 被破坏: %#v", wsOpts["headers"])
	}
	alpn, ok := proxy["alpn"].([]any)
	if !ok || len(alpn) != 2 {
		t.Fatalf("alpn 被破坏: %#v", proxy["alpn"])
	}
}

// TestNormalizeProxyFields_NestedContainerNilDropped 嵌套层里的空容器同样要清掉。
func TestNormalizeProxyFields_NestedContainerNilDropped(t *testing.T) {
	proxy := map[string]any{
		"name": "节点E",
		"ws-opts": map[string]any{
			"path":    "/ws",
			"headers": nil,
		},
	}

	normalizeProxyFields(proxy)

	wsOpts, ok := proxy["ws-opts"].(map[string]any)
	if !ok {
		t.Fatalf("ws-opts 应保留: %#v", proxy["ws-opts"])
	}
	if v, exists := wsOpts["headers"]; exists {
		t.Fatalf("嵌套 headers 为 nil 时应删除，实际为 %#v", v)
	}
	if wsOpts["path"] != "/ws" {
		t.Fatalf("同层 path 被误删: %#v", wsOpts)
	}
}

// TestNormalizeProxyFields_YAMLOutputHasNoStringWsOpts 端到端校验：
// 规范化后序列化出的 YAML 不再含 `ws-opts: ""`，且能被重新解析成合法结构。
func TestNormalizeProxyFields_YAMLOutputHasNoStringWsOpts(t *testing.T) {
	raw := `{"name":"节点F","type":"vmess","server":"f.example.com","port":443,"uuid":"u","network":"ws","ws-opts":null}`

	var proxy map[string]any
	if err := json.Unmarshal([]byte(raw), &proxy); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	normalizeProxyFields(proxy)

	out, err := yaml.Marshal(map[string]any{"proxies": []map[string]any{proxy}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), `ws-opts: ""`) || strings.Contains(string(out), "ws-opts: ''") {
		t.Fatalf("输出仍含字符串型 ws-opts:\n%s", out)
	}

	// 回读确认结构合法：ws-opts 要么不存在，要么是 map
	var back map[string]any
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	proxies, _ := back["proxies"].([]any)
	if len(proxies) != 1 {
		t.Fatalf("proxies 数量异常: %#v", back["proxies"])
	}
	first, _ := proxies[0].(map[string]any)
	if v, exists := first["ws-opts"]; exists {
		if _, isMap := v.(map[string]any); !isMap {
			t.Fatalf("ws-opts 存在但不是 map: %T", v)
		}
	}
}

// TestNormalizeProxyFieldsInList 批量入口。
func TestNormalizeProxyFieldsInList(t *testing.T) {
	proxies := []map[string]any{
		{"name": "n1", "ws-opts": nil},
		{"name": "n2", "grpc-opts": ""},
		{"name": "n3", "short-id": nil},
	}

	for _, p := range proxies {
		normalizeProxyFields(p)
	}

	if _, exists := proxies[0]["ws-opts"]; exists {
		t.Fatal("n1 的 ws-opts 应被删除")
	}
	if _, exists := proxies[1]["grpc-opts"]; exists {
		t.Fatal("n2 的 grpc-opts 应被删除")
	}
	if proxies[2]["short-id"] != "" {
		t.Fatalf("n3 的 short-id 应为空字符串: %#v", proxies[2]["short-id"])
	}
}

// TestNormalizeProxyFields_NilMapSafe 防御 nil map。
func TestNormalizeProxyFields_NilMapSafe(t *testing.T) {
	normalizeProxyFields(nil)
}

// TestNormalizeClashConfigJSON 覆盖手动保存入口：前端提交的 JSON 里
// "ws-opts": null 应在入库前被剔除，避免坏值落到数据库。
func TestNormalizeClashConfigJSON(t *testing.T) {
	raw := `{"name":"手动节点","type":"vmess","server":"m.example.com","port":443,` +
		`"uuid":"u-9","network":"ws","ws-opts":null,"short-id":null}`

	got := normalizeClashConfigJSON(raw)

	var cfg map[string]any
	if err := json.Unmarshal([]byte(got), &cfg); err != nil {
		t.Fatalf("规范化后的 JSON 无法解析: %v (%s)", err, got)
	}
	if v, exists := cfg["ws-opts"]; exists {
		t.Fatalf("ws-opts 应被删除，实际为 %#v", v)
	}
	if v, ok := cfg["short-id"].(string); !ok || v != "" {
		t.Fatalf("short-id 应为空字符串，实际为 %#v", cfg["short-id"])
	}
	if cfg["name"] != "手动节点" || cfg["network"] != "ws" {
		t.Fatalf("正常字段被误改: %#v", cfg)
	}
}

// TestNormalizeClashConfigJSON_InvalidInputUnchanged 非法 JSON 或空串应原样返回，
// 保证既有的错误处理分支（返回 400）行为不变。
func TestNormalizeClashConfigJSON_InvalidInputUnchanged(t *testing.T) {
	for _, raw := range []string{"", "not-json", `{"broken":`} {
		if got := normalizeClashConfigJSON(raw); got != raw {
			t.Fatalf("非法输入 %q 应原样返回，实际为 %q", raw, got)
		}
	}
}

// TestNormalizeProxyFields_EndToEnd 模拟真实故障场景：
// 数据库里存着 91 个节点，其中一个的 ws-opts 已被历史 bug 写成 "",
// 经过规范化后生成的 YAML 不应再出现 mihomo 无法解析的标量 ws-opts。
func TestNormalizeProxyFields_EndToEnd(t *testing.T) {
	proxies := make([]map[string]any, 0, 91)
	for i := 0; i < 91; i++ {
		p := map[string]any{
			"name":    "节点" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			"type":    "vmess",
			"server":  "s.example.com",
			"port":    443,
			"uuid":    "u",
			"network": "ws",
			"ws-opts": map[string]any{"path": "/ws"},
		}
		if i == 90 {
			// 第 91 个节点：历史 bug 留下的坏值
			p["ws-opts"] = ""
		}
		proxies = append(proxies, p)
	}

	for _, p := range proxies {
		normalizeProxyFields(p)
	}

	out, err := yaml.Marshal(map[string]any{"proxies": proxies})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if strings.Contains(string(out), `ws-opts: ""`) || strings.Contains(string(out), "ws-opts: ''") {
		t.Fatalf("输出仍含标量 ws-opts，mihomo 会解析失败:\n%s", out)
	}

	// 回读校验：所有存在的 ws-opts 都必须是 map
	var back map[string]any
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	list, _ := back["proxies"].([]any)
	if len(list) != 91 {
		t.Fatalf("节点数应为 91，实际 %d", len(list))
	}
	for i, item := range list {
		pm, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("proxies[%d] 不是 map", i)
		}
		if v, exists := pm["ws-opts"]; exists {
			if _, isMap := v.(map[string]any); !isMap {
				t.Fatalf("proxies[%d] 的 ws-opts 不是 map: %#v", i, v)
			}
		}
	}
	// 第 91 个节点的坏值应已被剔除
	if pm, ok := list[90].(map[string]any); ok {
		if _, exists := pm["ws-opts"]; exists {
			t.Fatalf("proxies[90] 的坏 ws-opts 未被剔除: %#v", pm["ws-opts"])
		}
	}
}

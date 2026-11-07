package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MMWOrg/mmwX-plugins/proxyparser/substore"
)

// issue #116: AnyTLS + REALITY。该组合只有 sing-box 支持,
// mihomo 官方声明不支持且不打算支持, 因此 clash/stash/loon 必须剔除该节点。
func TestAnyTLSRealityIssue116(t *testing.T) {
	uri := "anytls://L16Dwc8pK4fp1xeOK@38.1.2.3:20000?sni=www.example.com&fp=chrome" +
		"&security=reality&pbk=0WxD-SzAKKrjqiubZJ3o&sid=e1f7bbf1#AnyTLS-REALITY"

	node, err := ParseProxyURL(uri)
	if err != nil {
		t.Fatalf("解析 anytls URI 失败: %v", err)
	}
	ro, ok := node["reality-opts"].(map[string]any)
	if !ok {
		t.Fatalf("解析结果缺少 reality-opts: %v", node)
	}
	if ro["public-key"] != "0WxD-SzAKKrjqiubZJ3o" || ro["short-id"] != "e1f7bbf1" {
		t.Errorf("reality-opts = %v", ro)
	}
	// 下游靠 network 标记剔除不支持该组合的客户端, 不能丢。
	if node["network"] != "tcp" {
		t.Errorf("带 REALITY 的 AnyTLS 应保留 network=tcp 标记, got=%v", node["network"])
	}

	// sing-box: 必须下发, 且 reality/utls 落进 tls 块, server_name 取 sni。
	sb, err := substore.GetDefaultFactory().GetProducer("sing-box")
	if err != nil {
		t.Fatalf("取 sing-box producer: %v", err)
	}
	raw, err := sb.Produce([]substore.Proxy{node}, "", nil)
	if err != nil {
		t.Fatalf("sing-box produce: %v", err)
	}
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(raw.(string)), &cfg); err != nil {
		t.Fatalf("解析 sing-box 输出: %v", err)
	}
	if len(cfg.Outbounds) != 1 {
		t.Fatalf("sing-box 应下发 1 个 outbound, got %d", len(cfg.Outbounds))
	}
	tls, ok := cfg.Outbounds[0]["tls"].(map[string]any)
	if !ok {
		t.Fatalf("sing-box outbound 缺少 tls 块: %v", cfg.Outbounds[0])
	}
	if tls["server_name"] != "www.example.com" {
		t.Errorf("server_name = %v, 期望取 sni www.example.com", tls["server_name"])
	}
	reality, ok := tls["reality"].(map[string]any)
	if !ok {
		t.Fatalf("sing-box tls 缺少 reality: %v", tls)
	}
	if reality["public_key"] != "0WxD-SzAKKrjqiubZJ3o" || reality["short_id"] != "e1f7bbf1" {
		t.Errorf("sing-box reality = %v", reality)
	}

	// clash/stash/loon: 必须剔除, 否则下发给不支持该组合的客户端会出错。
	for _, name := range []string{"clashmeta", "stash", "loon"} {
		prod, err := substore.GetDefaultFactory().GetProducer(name)
		if err != nil {
			t.Fatalf("取 %s producer: %v", name, err)
		}
		out, err := prod.Produce([]substore.Proxy{node}, "", nil)
		if err != nil {
			t.Fatalf("%s produce: %v", name, err)
		}
		if s, _ := out.(string); strings.Contains(strings.ToLower(s), "anytls") {
			t.Errorf("%s 不支持 AnyTLS+REALITY, 不应下发该节点: %s", name, s)
		}
	}
}

// 普通 AnyTLS(无 REALITY)不受影响, 各客户端照常下发。
func TestAnyTLSWithoutRealityStillDelivered(t *testing.T) {
	node, err := ParseProxyURL("anytls://pw@38.1.2.3:20000?sni=www.example.com#AnyTLS")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if _, ok := node["network"]; ok {
		t.Errorf("普通 AnyTLS 不应带 network 标记, got=%v", node["network"])
	}
	for _, name := range []string{"clashmeta", "stash", "sing-box"} {
		prod, _ := substore.GetDefaultFactory().GetProducer(name)
		out, err := prod.Produce([]substore.Proxy{node}, "", nil)
		if err != nil {
			t.Fatalf("%s produce: %v", name, err)
		}
		if s, _ := out.(string); !strings.Contains(strings.ToLower(s), "anytls") {
			t.Errorf("%s 应正常下发普通 AnyTLS 节点: %s", name, s)
		}
	}
}

// 往返: AnyTLS+REALITY 节点导出成 anytls:// 再解析回来, REALITY 不能丢。
func TestAnyTLSRealityURIRoundTrip(t *testing.T) {
	const pbk, sid = "0WxD-SzAKKrjqiubZJ3o", "e1f7bbf1"
	node, err := ParseProxyURL("anytls://pw@38.1.2.3:20000?sni=www.example.com" +
		"&security=reality&pbk=" + pbk + "&sid=" + sid + "#N")
	if err != nil {
		t.Fatalf("首次解析失败: %v", err)
	}
	prod, err := substore.GetDefaultFactory().GetProducer("uri")
	if err != nil {
		t.Fatalf("取 uri producer: %v", err)
	}
	raw, err := prod.Produce([]substore.Proxy{node}, "", nil)
	if err != nil {
		t.Fatalf("导出 URI 失败: %v", err)
	}
	uri := strings.TrimSpace(raw.(string))

	back, err := ParseProxyURL(uri)
	if err != nil {
		t.Fatalf("回解析失败: %v (uri=%s)", err, uri)
	}
	ro, ok := back["reality-opts"].(map[string]any)
	if !ok {
		t.Fatalf("往返后 reality-opts 丢失: %v (uri=%s)", back, uri)
	}
	if ro["public-key"] != pbk || ro["short-id"] != sid {
		t.Errorf("往返后 reality-opts = %v, 期望 pbk=%s sid=%s", ro, pbk, sid)
	}
	if back["network"] != "tcp" {
		t.Errorf("往返后应保留 network=tcp 标记, got=%v", back["network"])
	}
}

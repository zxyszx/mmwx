package handler

import "encoding/json"

// 代理节点字段规范化。
//
// 背景：早期为修复 `short-id: null` 被 YAML 输出成 `<nil>` 的问题，代码里用
// convertNilToEmptyString 把 map 中所有 nil 一律替换成 ""。这个做法对标量字段有效，
// 但对 ws-opts / grpc-opts / reality-opts / headers 这类必须是 map 或 list 的字段是错的：
// 一旦订阅源给出 `ws-opts:`（空值），就会被写成 `ws-opts: ""`，mihomo 解析时报
// `proxy 91: 'ws-opts' expected a map, got 'string'`，整份订阅直接加载失败。
//
// 正确做法是按字段的期望类型区别对待：
//   - 标量字段（short-id、sni 等）：nil -> ""，保持原有修复意图
//   - 容器字段（ws-opts 等）：nil 或空字符串 -> 删除该键，让 mihomo 走默认值
//
// 删除而非补空 map 的理由：mihomo 对缺失的传输层配置有明确默认行为，
// 而 `ws-opts: {}` 在部分客户端上会覆盖掉本可推导的默认 path。

// containerProxyFields 是在 Clash/mihomo 配置中必须为 map 的字段。
// 清单依据 proxyparser/substore 中通过 GetMap(proxy, X) 读取的字段，
// 以及 mihomo 文档中的传输层与插件配置节。
var containerProxyFields = map[string]struct{}{
	// 传输层
	"ws-opts":        {},
	"grpc-opts":      {},
	"h2-opts":        {},
	"http-opts":      {},
	"splithttp-opts": {},
	"xhttp-opts":     {},
	// TLS / 安全
	"reality-opts": {},
	"ech-opts":     {},
	// 混淆与插件
	"obfs-opts":   {},
	"plugin-opts": {},
	"brutal-opts": {},
	// 其他嵌套结构
	"headers":       {},
	"ws-headers":    {},
	"extra-headers": {},
	"smux":          {},
	"transport":     {},
}

// listProxyFields 是在 Clash/mihomo 配置中必须为数组的字段。
var listProxyFields = map[string]struct{}{
	"alpn":                {},
	"addresses":           {},
	"allowed-ips":         {},
	"host-key":            {},
	"host-key-algorithms": {},
}

// isContainerProxyField 判断字段是否必须为 map。
func isContainerProxyField(key string) bool {
	_, ok := containerProxyFields[key]
	return ok
}

// isListProxyField 判断字段是否必须为数组。
func isListProxyField(key string) bool {
	_, ok := listProxyFields[key]
	return ok
}

// normalizeProxyFields 递归规范化代理节点配置中的空值。
//
// 对每个键：
//   - 若是容器字段（期望 map/list）且值为 nil 或空字符串，删除该键；
//     这既阻止新的坏值产生，也能自愈数据库里已存的 `"ws-opts": ""`。
//   - 若是标量字段且值为 nil，替换为 ""（沿用 short-id 的历史修复）。
//   - 其余情况递归进入嵌套 map 与数组。
func normalizeProxyFields(m map[string]any) {
	if m == nil {
		return
	}
	for k, v := range m {
		// 容器字段：空值一律删键，交给客户端默认行为
		if isContainerProxyField(k) || isListProxyField(k) {
			if v == nil {
				delete(m, k)
				continue
			}
			if s, ok := v.(string); ok && s == "" {
				delete(m, k)
				continue
			}
		}

		switch typed := v.(type) {
		case nil:
			// 标量字段的 nil：保持历史行为，转成空字符串
			m[k] = ""
		case map[string]any:
			normalizeProxyFields(typed)
		case []any:
			normalizeProxyList(typed)
		}
	}
}

// normalizeProxyList 规范化数组元素：nil 转 ""，嵌套 map 递归处理。
func normalizeProxyList(items []any) {
	for i, item := range items {
		switch typed := item.(type) {
		case nil:
			items[i] = ""
		case map[string]any:
			normalizeProxyFields(typed)
		case []any:
			normalizeProxyList(typed)
		}
	}
}

// normalizeClashConfigJSON 对节点的 ClashConfig JSON 字符串做规范化后回写。
//
// 用于节点手动创建/编辑入口：前端可能提交 `"ws-opts": null` 或 `"ws-opts": ""`，
// 若原样入库，后续生成订阅时就会产出 mihomo 无法解析的 `ws-opts: ""`。
// 解析失败时返回原字符串，由调用方原有的校验逻辑负责报错。
func normalizeClashConfigJSON(raw string) string {
	if raw == "" {
		return raw
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw
	}
	normalizeProxyFields(cfg)
	normalized, err := json.Marshal(cfg)
	if err != nil {
		return raw
	}
	return string(normalized)
}

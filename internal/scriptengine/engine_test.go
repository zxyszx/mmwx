package scriptengine

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunPostFetch(t *testing.T) {
	script := `function main(config) {
		config.dns = { enable: true };
		config.proxies = config.proxies.filter(function(p) { return p.name !== 'bad'; });
		return config;
	}`

	config := map[string]interface{}{
		"proxies": []interface{}{
			map[string]interface{}{"name": "good", "type": "ss"},
			map[string]interface{}{"name": "bad", "type": "ss"},
		},
	}

	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dns, ok := result["dns"].(map[string]interface{})
	if !ok {
		t.Fatal("dns should be a map")
	}
	if dns["enable"] != true {
		t.Errorf("dns.enable = %v, want true", dns["enable"])
	}

	proxies, ok := result["proxies"].([]interface{})
	if !ok {
		t.Fatalf("proxies should be an array, got %T", result["proxies"])
	}
	if len(proxies) != 1 {
		t.Errorf("len(proxies) = %d, want 1", len(proxies))
	}
}

func TestRunPostFetch_ReturnNil(t *testing.T) {
	script := `function main(config) { return null; }`
	config := map[string]interface{}{"key": "value"}

	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected original config to be returned when script returns null")
	}
}

func TestRunPostFetch_ReturnNonObject(t *testing.T) {
	script := `function main(config) { return 42; }`
	config := map[string]interface{}{}

	_, err := RunPostFetch(context.Background(), script, config)
	if err == nil {
		t.Fatal("expected error for non-object return")
	}
	if !strings.Contains(err.Error(), "must return an object") {
		t.Errorf("error = %v, want 'must return an object'", err)
	}
}

func TestRunPreSaveNodes(t *testing.T) {
	script := `function main(proxies) {
		return proxies.map(function(p) {
			p.name = 'prefix_' + p.name;
			p['skip-cert-verify'] = 'true';
			return p;
		});
	}`

	proxies := []map[string]interface{}{
		{"name": "node1", "type": "ss", "server": "1.1.1.1"},
		{"name": "node2", "type": "vmess", "server": "2.2.2.2"},
	}

	result, err := RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
	if result[0]["name"] != "prefix_node1" {
		t.Errorf("result[0].name = %v, want prefix_node1", result[0]["name"])
	}
	if result[1]["name"] != "prefix_node2" {
		t.Errorf("result[1].name = %v, want prefix_node2", result[1]["name"])
	}
	if result[0]["skip-cert-verify"] != "true" {
		t.Errorf("result[0].skip-cert-verify = %v, want true", result[0]["skip-cert-verify"])
	}
}

func TestRunPreSaveNodes_Filter(t *testing.T) {
	script := `function main(proxies) {
		return proxies.filter(function(p) { return p.type !== 'ssr'; });
	}`

	proxies := []map[string]interface{}{
		{"name": "node1", "type": "ss"},
		{"name": "node2", "type": "ssr"},
		{"name": "node3", "type": "vmess"},
	}

	result, err := RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
	if result[0]["name"] != "node1" {
		t.Errorf("result[0].name = %v, want node1", result[0]["name"])
	}
	if result[1]["name"] != "node3" {
		t.Errorf("result[1].name = %v, want node3", result[1]["name"])
	}
}

func TestRunPreSaveNodes_ReturnNil(t *testing.T) {
	script := `function main(proxies) { return null; }`
	proxies := []map[string]interface{}{
		{"name": "node1", "type": "ss"},
	}

	result, err := RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected original proxies returned when script returns null")
	}
}

func TestRunPreSaveNodes_ReturnNonArray(t *testing.T) {
	script := `function main(proxies) { return {}; }`
	proxies := []map[string]interface{}{
		{"name": "node1", "type": "ss"},
	}

	_, err := RunPreSaveNodes(context.Background(), script, proxies)
	if err == nil {
		t.Fatal("expected error for non-array return")
	}
	if !strings.Contains(err.Error(), "must return an array") {
		t.Errorf("error = %v, want 'must return an array'", err)
	}
}

func TestRunPostFetch_Timeout(t *testing.T) {
	script := `function main(config) {
		while(true) {}
		return config;
	}`

	config := map[string]interface{}{}
	_, err := RunPostFetch(context.Background(), script, config)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error = %v, want timeout", err)
	}
}

func TestRunPostFetch_ContextCancel(t *testing.T) {
	script := `function main(config) {
		while(true) {}
		return config;
	}`

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	config := map[string]interface{}{}
	_, err := RunPostFetch(ctx, script, config)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRunPostFetch_SyntaxError(t *testing.T) {
	script := `function main(config { return config; }`

	config := map[string]interface{}{}
	_, err := RunPostFetch(context.Background(), script, config)
	if err == nil {
		t.Fatal("expected syntax error")
	}
	if !strings.Contains(err.Error(), "script error") {
		t.Errorf("error = %v, want 'script error'", err)
	}
}

func TestRunPostFetch_RuntimeError(t *testing.T) {
	script := `function main(config) {
		config.proxies.nonExistentMethod();
		return config;
	}`

	config := map[string]interface{}{}
	_, err := RunPostFetch(context.Background(), script, config)
	if err == nil {
		t.Fatal("expected runtime error")
	}
}

func TestRunPreSaveNodes_EmptyInput(t *testing.T) {
	script := `function main(proxies) { return proxies; }`

	result, err := RunPreSaveNodes(context.Background(), script, []map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestConsoleLog(t *testing.T) {
	script := `function main(config) {
		console.log('hello', 'world');
		console.warn('warning message');
		console.error('error message');
		return config;
	}`

	config := map[string]interface{}{"key": "value"}
	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("result[key] = %v, want value", result["key"])
	}
}

func TestConsoleLog_DifferentTypes(t *testing.T) {
	script := `function main(proxies) {
		console.log('count:', proxies.length);
		console.log('first:', proxies[0].name, 'type:', proxies[0].type);
		console.log('object:', {a: 1, b: 2});
		console.log('array:', [1, 2, 3]);
		console.log('null:', null, 'undefined:', undefined, 'bool:', true);
		return proxies;
	}`

	proxies := []map[string]interface{}{
		{"name": "node1", "type": "ss"},
	}

	result, err := RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("len(result) = %d, want 1", len(result))
	}
}

func TestProduce_URI(t *testing.T) {
	script := `function main(config) {
		var result = produce(config.proxies, 'uri');
		config.uri_output = result;
		return config;
	}`

	config := map[string]interface{}{
		"proxies": []interface{}{
			map[string]interface{}{
				"name":   "test-ss",
				"type":   "ss",
				"server": "1.2.3.4",
				"port":   int64(8388),
				"cipher": "aes-256-gcm",
				"password": "testpass",
			},
		},
	}

	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output, ok := result["uri_output"].(string)
	if !ok {
		t.Fatalf("uri_output should be string, got %T", result["uri_output"])
	}
	if output == "" {
		t.Error("uri_output should not be empty")
	}
}

func TestProduce_InvalidFormat(t *testing.T) {
	script := `function main(config) {
		var result = produce(config.proxies, 'nonexistent_format');
		return config;
	}`

	config := map[string]interface{}{
		"proxies": []interface{}{
			map[string]interface{}{
				"name":   "test",
				"type":   "ss",
				"server": "1.2.3.4",
				"port":   int64(8388),
			},
		},
	}

	_, err := RunPostFetch(context.Background(), script, config)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestProduce_TooFewArguments(t *testing.T) {
	script := `function main(config) {
		produce(config.proxies);
		return config;
	}`

	config := map[string]interface{}{
		"proxies": []interface{}{},
	}

	_, err := RunPostFetch(context.Background(), script, config)
	if err == nil {
		t.Fatal("expected error for too few arguments")
	}
	if !strings.Contains(err.Error(), "requires 2 arguments") {
		t.Errorf("error = %v, want 'requires 2 arguments'", err)
	}
}

func TestProduce_InvalidFirstArgument(t *testing.T) {
	script := `function main(config) {
		produce("not_an_array", 'uri');
		return config;
	}`

	config := map[string]interface{}{}

	_, err := RunPostFetch(context.Background(), script, config)
	if err == nil {
		t.Fatal("expected error for invalid first argument")
	}
	if !strings.Contains(err.Error(), "must be an array") {
		t.Errorf("error = %v, want 'must be an array'", err)
	}
}

func TestProduce_EmptyProxies(t *testing.T) {
	script := `function main(config) {
		var result = produce([], 'uri');
		config.output = result;
		return config;
	}`

	config := map[string]interface{}{}

	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["output"] == nil {
		t.Error("expected non-nil output for empty proxies")
	}
}

func TestProduce_InPreSaveNodes(t *testing.T) {
	script := `function main(proxies) {
		console.log('converting', proxies.length, 'proxies to surge format');
		var surgeResult = produce(proxies, 'surge');
		console.log('surge result:', surgeResult);
		return proxies;
	}`

	proxies := []map[string]interface{}{
		{
			"name":     "test-ss",
			"type":     "ss",
			"server":   "1.2.3.4",
			"port":     int64(8388),
			"cipher":   "aes-256-gcm",
			"password": "testpass",
		},
	}

	result, err := RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
}

func TestProduce_MultipleFormats(t *testing.T) {
	script := `function main(config) {
		var proxies = config.proxies;
		config.surge = produce(proxies, 'surge');
		config.clash = produce(proxies, 'clash');
		config.uri = produce(proxies, 'uri');
		return config;
	}`

	config := map[string]interface{}{
		"proxies": []interface{}{
			map[string]interface{}{
				"name":     "test-ss",
				"type":     "ss",
				"server":   "1.2.3.4",
				"port":     int64(8388),
				"cipher":   "aes-256-gcm",
				"password": "testpass",
			},
		},
	}

	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["surge"] == nil {
		t.Error("surge output should not be nil")
	}
	if result["clash"] == nil {
		t.Error("clash output should not be nil")
	}
	if result["uri"] == nil {
		t.Error("uri output should not be nil")
	}
}

func TestScriptModifiesAndProduces(t *testing.T) {
	script := `function main(proxies) {
		// Modify nodes then produce
		var modified = proxies.map(function(p) {
			p.name = 'modified_' + p.name;
			return p;
		});
		// Verify produce works with modified proxies
		var uriResult = produce(modified, 'uri');
		console.log('produced URI:', uriResult);
		return modified;
	}`

	proxies := []map[string]interface{}{
		{
			"name":     "node1",
			"type":     "ss",
			"server":   "1.2.3.4",
			"port":     int64(8388),
			"cipher":   "aes-256-gcm",
			"password": "pass",
		},
	}

	result, err := RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0]["name"] != "modified_node1" {
		t.Errorf("result[0].name = %v, want modified_node1", result[0]["name"])
	}
}

func TestRunPostFetch_AddRules(t *testing.T) {
	script := `function main(config) {
		var rules = config.rules ? Array.prototype.slice.call(config.rules) : [];
		rules.unshift('DOMAIN-SUFFIX,example.com,DIRECT');
		rules.unshift('DOMAIN-SUFFIX,test.com,PROXY');
		config.rules = rules;
		return config;
	}`

	config := map[string]interface{}{
		"rules": []interface{}{"MATCH,DIRECT"},
	}

	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rules, ok := result["rules"].([]interface{})
	if !ok {
		t.Fatalf("rules should be an array, got %T", result["rules"])
	}
	if len(rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(rules))
	}
	if rules[0] != "DOMAIN-SUFFIX,test.com,PROXY" {
		t.Errorf("rules[0] = %v, want DOMAIN-SUFFIX,test.com,PROXY", rules[0])
	}
	if rules[2] != "MATCH,DIRECT" {
		t.Errorf("rules[2] = %v, want MATCH,DIRECT", rules[2])
	}
}

func TestRunPostFetch_ModifyProxyGroups(t *testing.T) {
	script := `function main(config) {
		if (config['proxy-groups']) {
			config['proxy-groups'].forEach(function(group) {
				if (group.type === 'url-test') {
					group.interval = 300;
					group.tolerance = 50;
				}
			});
		}
		return config;
	}`

	config := map[string]interface{}{
		"proxy-groups": []interface{}{
			map[string]interface{}{"name": "auto", "type": "url-test", "interval": int64(600)},
			map[string]interface{}{"name": "manual", "type": "select"},
		},
	}

	result, err := RunPostFetch(context.Background(), script, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	groups := result["proxy-groups"].([]interface{})
	autoGroup := groups[0].(map[string]interface{})
	if autoGroup["interval"] != int64(300) {
		t.Errorf("interval = %v, want 300", autoGroup["interval"])
	}
	if autoGroup["tolerance"] != int64(50) {
		t.Errorf("tolerance = %v, want 50", autoGroup["tolerance"])
	}

	selectGroup := groups[1].(map[string]interface{})
	if selectGroup["type"] != "select" {
		t.Errorf("select group type changed unexpectedly")
	}
}

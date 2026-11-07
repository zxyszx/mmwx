package handler

import (
	"context"
	"strings"
	"testing"

	"miaomiaowu/internal/scriptengine"

	"gopkg.in/yaml.v3"
)

// TestRunPostFetchScript_ModifiesDNS verifies the post_fetch hook modifies config
// and the output YAML reflects the changes.
func TestRunPostFetchScript_ModifiesDNS(t *testing.T) {
	h := &SubscriptionHandler{}

	inputYAML := `proxies:
  - name: node1
    type: ss
    server: 1.2.3.4
    port: 8388
dns:
  enable: false
  nameserver:
    - 8.8.8.8
`

	script := `function main(config) {
		config.dns.enable = true;
		config.dns.nameserver = ['223.5.5.5', '1.1.1.1'];
		return config;
	}`

	result, err := h.runPostFetchScript(context.Background(), script, []byte(inputYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output map[string]interface{}
	if err := yaml.Unmarshal(result, &output); err != nil {
		t.Fatalf("failed to parse output YAML: %v", err)
	}

	dns, ok := output["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("dns should be a map, got %T", output["dns"])
	}
	if dns["enable"] != true {
		t.Errorf("dns.enable = %v, want true", dns["enable"])
	}

	nameservers, ok := dns["nameserver"].([]interface{})
	if !ok {
		t.Fatalf("nameserver should be an array, got %T", dns["nameserver"])
	}
	if len(nameservers) != 2 || nameservers[0] != "223.5.5.5" {
		t.Errorf("nameservers = %v, want [223.5.5.5, 1.1.1.1]", nameservers)
	}
}

// TestRunPostFetchScript_FilterProxies verifies nodes can be filtered by the script.
func TestRunPostFetchScript_FilterProxies(t *testing.T) {
	h := &SubscriptionHandler{}

	inputYAML := `proxies:
  - name: good-node
    type: ss
    server: 1.1.1.1
    port: 443
  - name: 过期节点
    type: ss
    server: 2.2.2.2
    port: 443
  - name: another-good
    type: vmess
    server: 3.3.3.3
    port: 443
`

	script := `function main(config) {
		if (config.proxies) {
			config.proxies = config.proxies.filter(function(p) {
				return p.name.indexOf('过期') === -1;
			});
		}
		console.log('filtered proxies count:', config.proxies.length);
		return config;
	}`

	result, err := h.runPostFetchScript(context.Background(), script, []byte(inputYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output map[string]interface{}
	if err := yaml.Unmarshal(result, &output); err != nil {
		t.Fatalf("failed to parse output YAML: %v", err)
	}

	proxies, ok := output["proxies"].([]interface{})
	if !ok {
		t.Fatalf("proxies should be array, got %T", output["proxies"])
	}
	if len(proxies) != 2 {
		t.Fatalf("len(proxies) = %d, want 2", len(proxies))
	}
}

// TestRunPostFetchScript_AddRules verifies rules can be prepended to config.
func TestRunPostFetchScript_AddRules(t *testing.T) {
	h := &SubscriptionHandler{}

	inputYAML := `proxies:
  - name: node1
    type: ss
    server: 1.1.1.1
    port: 443
rules:
  - MATCH,DIRECT
`

	script := `function main(config) {
		var rules = [];
		rules.push('DOMAIN-SUFFIX,example.com,PROXY');
		for (var i = 0; i < config.rules.length; i++) {
			rules.push(config.rules[i]);
		}
		config.rules = rules;
		return config;
	}`

	result, err := h.runPostFetchScript(context.Background(), script, []byte(inputYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output map[string]interface{}
	if err := yaml.Unmarshal(result, &output); err != nil {
		t.Fatalf("failed to parse output YAML: %v", err)
	}

	rules, ok := output["rules"].([]interface{})
	if !ok {
		t.Fatalf("rules should be array, got %T", output["rules"])
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	if rules[0] != "DOMAIN-SUFFIX,example.com,PROXY" {
		t.Errorf("rules[0] = %v, want DOMAIN-SUFFIX,example.com,PROXY", rules[0])
	}
	if rules[1] != "MATCH,DIRECT" {
		t.Errorf("rules[1] = %v, want MATCH,DIRECT", rules[1])
	}
}

// TestRunPostFetchScript_ClashConfigModified verifies that even for clash output
// (no format conversion), the script modifies the YAML that becomes the final output.
func TestRunPostFetchScript_ClashConfigModified(t *testing.T) {
	h := &SubscriptionHandler{}

	inputYAML := `mixed-port: 7890
proxies:
  - name: node1
    type: ss
    server: 1.1.1.1
    port: 443
proxy-groups:
  - name: auto
    type: url-test
    interval: 600
`

	script := `function main(config) {
		config['mixed-port'] = 7891;
		if (config['proxy-groups']) {
			for (var i = 0; i < config['proxy-groups'].length; i++) {
				var group = config['proxy-groups'][i];
				if (group.type === 'url-test') {
					group.interval = 300;
					group.tolerance = 50;
				}
			}
		}
		return config;
	}`

	result, err := h.runPostFetchScript(context.Background(), script, []byte(inputYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output map[string]interface{}
	if err := yaml.Unmarshal(result, &output); err != nil {
		t.Fatalf("failed to parse output YAML: %v", err)
	}

	if output["mixed-port"] != 7891 {
		t.Errorf("mixed-port = %v, want 7891", output["mixed-port"])
	}

	groups, ok := output["proxy-groups"].([]interface{})
	if !ok {
		t.Fatalf("proxy-groups should be array, got %T", output["proxy-groups"])
	}
	autoGroup, ok := groups[0].(map[string]interface{})
	if !ok {
		t.Fatalf("group should be map, got %T", groups[0])
	}
	if autoGroup["interval"] != 300 {
		t.Errorf("interval = %v, want 300", autoGroup["interval"])
	}
	if autoGroup["tolerance"] != 50 {
		t.Errorf("tolerance = %v, want 50", autoGroup["tolerance"])
	}
}

// TestRunPostFetchScript_ScriptError verifies that a script error is returned.
func TestRunPostFetchScript_ScriptError(t *testing.T) {
	h := &SubscriptionHandler{}

	inputYAML := `proxies:
  - name: node1
    type: ss
`
	script := `function main(config) { throw new Error("test error"); }`

	_, err := h.runPostFetchScript(context.Background(), script, []byte(inputYAML))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Errorf("error = %v, want to contain 'test error'", err)
	}
}

// TestRunPostFetchScript_InvalidYAML verifies that invalid input YAML returns error.
func TestRunPostFetchScript_InvalidYAML(t *testing.T) {
	h := &SubscriptionHandler{}

	_, err := h.runPostFetchScript(context.Background(), `function main(c) { return c; }`, []byte(":::invalid"))
	if err == nil {
		t.Fatal("expected parse error for invalid YAML")
	}
}

// TestPreSaveNodes_ModifiesNodes verifies the pre_save_nodes hook pipeline:
// yamlNodeToProxies → RunPreSaveNodes → proxiesToYamlNode preserves modifications.
func TestPreSaveNodes_ModifiesNodes(t *testing.T) {
	inputYAML := `- name: node1
  type: ss
  server: 1.1.1.1
  port: "443"
- name: node2
  type: vmess
  server: 2.2.2.2
  port: "8080"
`

	var seqNode yaml.Node
	if err := yaml.Unmarshal([]byte(inputYAML), &seqNode); err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	// seqNode is a document node, get the sequence inside
	proxiesNode := seqNode.Content[0]

	// Convert to Go maps (simulates yamlNodeToProxies)
	proxies := yamlNodeToProxies(proxiesNode)
	if len(proxies) != 2 {
		t.Fatalf("len(proxies) = %d, want 2", len(proxies))
	}

	// Run the override script
	script := `function main(proxies) {
		return proxies.map(function(p) {
			p.name = 'PREFIX_' + p.name;
			p['skip-cert-verify'] = 'true';
			return p;
		});
	}`

	modified, err := scriptengine.RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("RunPreSaveNodes error: %v", err)
	}

	// Verify script modifications
	if len(modified) != 2 {
		t.Fatalf("len(modified) = %d, want 2", len(modified))
	}
	if modified[0]["name"] != "PREFIX_node1" {
		t.Errorf("modified[0].name = %v, want PREFIX_node1", modified[0]["name"])
	}
	if modified[1]["name"] != "PREFIX_node2" {
		t.Errorf("modified[1].name = %v, want PREFIX_node2", modified[1]["name"])
	}
	if modified[0]["skip-cert-verify"] != "true" {
		t.Errorf("modified[0].skip-cert-verify = %v, want true", modified[0]["skip-cert-verify"])
	}

	// Convert back to YAML node (simulates proxiesToYamlNode)
	outputNode := proxiesToYamlNode(modified)

	// Marshal and verify the final output
	outputBytes, err := yaml.Marshal(outputNode)
	if err != nil {
		t.Fatalf("failed to marshal output: %v", err)
	}

	output := string(outputBytes)
	if !strings.Contains(output, "PREFIX_node1") {
		t.Errorf("output should contain PREFIX_node1, got:\n%s", output)
	}
	if !strings.Contains(output, "PREFIX_node2") {
		t.Errorf("output should contain PREFIX_node2, got:\n%s", output)
	}
	if !strings.Contains(output, "skip-cert-verify") {
		t.Errorf("output should contain skip-cert-verify, got:\n%s", output)
	}
}

// TestPreSaveNodes_FilterNodes verifies that the script can remove nodes.
func TestPreSaveNodes_FilterNodes(t *testing.T) {
	inputYAML := `- name: keep-node
  type: ss
  server: 1.1.1.1
  port: "443"
- name: remove-ssr
  type: ssr
  server: 2.2.2.2
  port: "8080"
- name: keep-vmess
  type: vmess
  server: 3.3.3.3
  port: "443"
`

	var seqNode yaml.Node
	if err := yaml.Unmarshal([]byte(inputYAML), &seqNode); err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	proxies := yamlNodeToProxies(seqNode.Content[0])

	script := `function main(proxies) {
		console.log('input count:', proxies.length);
		var filtered = proxies.filter(function(p) {
			var keep = p.type !== 'ssr';
			if (!keep) console.log('removing:', p.name);
			return keep;
		});
		console.log('output count:', filtered.length);
		return filtered;
	}`

	modified, err := scriptengine.RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("RunPreSaveNodes error: %v", err)
	}

	if len(modified) != 2 {
		t.Fatalf("len(modified) = %d, want 2", len(modified))
	}
	if modified[0]["name"] != "keep-node" {
		t.Errorf("modified[0].name = %v, want keep-node", modified[0]["name"])
	}
	if modified[1]["name"] != "keep-vmess" {
		t.Errorf("modified[1].name = %v, want keep-vmess", modified[1]["name"])
	}

	// Convert back and verify
	outputNode := proxiesToYamlNode(modified)
	outputBytes, _ := yaml.Marshal(outputNode)
	output := string(outputBytes)

	if strings.Contains(output, "remove-ssr") {
		t.Errorf("output should NOT contain remove-ssr, got:\n%s", output)
	}
	if !strings.Contains(output, "keep-node") {
		t.Errorf("output should contain keep-node, got:\n%s", output)
	}
}

// TestPreSaveNodes_ModifyServerAndPort verifies critical fields can be modified.
func TestPreSaveNodes_ModifyServerAndPort(t *testing.T) {
	inputYAML := `- name: node1
  type: ss
  server: old-server.example.com
  port: "443"
  password: secret
`

	var seqNode yaml.Node
	if err := yaml.Unmarshal([]byte(inputYAML), &seqNode); err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	proxies := yamlNodeToProxies(seqNode.Content[0])

	script := `function main(proxies) {
		return proxies.map(function(p) {
			p.server = 'new-server.example.com';
			p.port = '8443';
			return p;
		});
	}`

	modified, err := scriptengine.RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("RunPreSaveNodes error: %v", err)
	}

	if modified[0]["server"] != "new-server.example.com" {
		t.Errorf("server = %v, want new-server.example.com", modified[0]["server"])
	}
	if modified[0]["port"] != "8443" {
		t.Errorf("port = %v, want 8443", modified[0]["port"])
	}
	// Verify original fields are preserved
	if modified[0]["password"] != "secret" {
		t.Errorf("password = %v, want secret (should be preserved)", modified[0]["password"])
	}
}

// TestPreSaveNodes_ScriptErrorSkips verifies that script errors don't corrupt data.
func TestPreSaveNodes_ScriptErrorSkips(t *testing.T) {
	proxies := []map[string]interface{}{
		{"name": "node1", "type": "ss", "server": "1.1.1.1", "port": "443"},
	}

	script := `function main(proxies) { throw new Error("intentional error"); }`

	_, err := scriptengine.RunPreSaveNodes(context.Background(), script, proxies)
	if err == nil {
		t.Fatal("expected error from script")
	}
	if !strings.Contains(err.Error(), "intentional error") {
		t.Errorf("error = %v, want to contain 'intentional error'", err)
	}
}

// TestPreSaveNodes_RoundTrip verifies the full pipeline preserves all proxy fields.
func TestPreSaveNodes_RoundTrip(t *testing.T) {
	inputYAML := `- name: test-trojan
  type: trojan
  server: trojan.example.com
  port: "443"
  password: trojan-pass
  sni: example.com
  skip-cert-verify: "false"
  udp: "true"
`

	var seqNode yaml.Node
	if err := yaml.Unmarshal([]byte(inputYAML), &seqNode); err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	proxies := yamlNodeToProxies(seqNode.Content[0])

	// Script that passes through without modification
	script := `function main(proxies) { return proxies; }`

	modified, err := scriptengine.RunPreSaveNodes(context.Background(), script, proxies)
	if err != nil {
		t.Fatalf("RunPreSaveNodes error: %v", err)
	}

	if len(modified) != 1 {
		t.Fatalf("len(modified) = %d, want 1", len(modified))
	}

	// Verify all fields preserved
	p := modified[0]
	expectations := map[string]string{
		"name":              "test-trojan",
		"type":              "trojan",
		"server":            "trojan.example.com",
		"port":              "443",
		"password":          "trojan-pass",
		"sni":               "example.com",
		"skip-cert-verify":  "false",
		"udp":               "true",
	}
	for key, want := range expectations {
		got, _ := p[key].(string)
		if got != want {
			t.Errorf("p[%s] = %v, want %s", key, p[key], want)
		}
	}

	// Convert back and verify YAML output
	outputNode := proxiesToYamlNode(modified)
	outputBytes, _ := yaml.Marshal(outputNode)
	output := string(outputBytes)

	for _, field := range []string{"trojan.example.com", "trojan-pass", "example.com"} {
		if !strings.Contains(output, field) {
			t.Errorf("output should contain %q, got:\n%s", field, output)
		}
	}
}

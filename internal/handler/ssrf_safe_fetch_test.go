package handler

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestValidateFetchURL(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/a", "http:///missing-host"} {
		if err := validateFetchURL(raw); err == nil {
			t.Fatalf("validateFetchURL(%q) unexpectedly succeeded", raw)
		}
	}
	if err := validateFetchURL("https://example.com/sub"); err != nil {
		t.Fatalf("valid URL rejected: %v", err)
	}
}

func TestSSRFSafeDialBlocksPrivateLiteral(t *testing.T) {
	dial := ssrfSafeDialContext(&net.Dialer{Timeout: time.Second})
	_, err := dial(context.Background(), "tcp", "127.0.0.1:80")
	if !errors.Is(err, errSSRFBlocked) {
		t.Fatalf("expected SSRF rejection, got %v", err)
	}
}

// 管理员白名单放行内网订阅源（issue #120）
func TestSSRFSafeDialAllowList(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	t.Cleanup(func() { SetSSRFAllowList("") })

	cases := []struct {
		allowList string
		addr      string
		allowed   bool
	}{
		{"", "127.0.0.1:" + port, false},
		{"127.0.0.0/8", "127.0.0.1:" + port, true},
		{"127.0.0.1", "127.0.0.1:" + port, true},
		{"10.0.0.0/8", "127.0.0.1:" + port, false},
		{"localhost", "localhost:" + port, true},
		{"*.example.internal\nlocalhost", "localhost:" + port, true},
		{"example.internal", "localhost:" + port, false},
	}
	dial := ssrfSafeDialContext(&net.Dialer{Timeout: time.Second})
	for _, tc := range cases {
		SetSSRFAllowList(tc.allowList)
		conn, err := dial(context.Background(), "tcp", tc.addr)
		if conn != nil {
			conn.Close()
		}
		if tc.allowed && err != nil {
			t.Errorf("allowList=%q addr=%s: expected allowed, got %v", tc.allowList, tc.addr, err)
		}
		if !tc.allowed && !errors.Is(err, errSSRFBlocked) {
			t.Errorf("allowList=%q addr=%s: expected SSRF rejection, got %v", tc.allowList, tc.addr, err)
		}
	}
}

func TestParseSSRFAllowList(t *testing.T) {
	list, invalid := parseSSRFAllowList("# 注释\nSub.Example.com, 192.168.1.10\n10.0.0.0/8\n*.lan.\nhttp://bad/path")
	if len(invalid) != 1 || invalid[0] != "http://bad/path" {
		t.Fatalf("invalid = %v", invalid)
	}
	for host, want := range map[string]bool{"sub.example.com": true, "a.sub.example.com": true, "example.com": false, "nas.lan": true, "evil-lan": false} {
		if got := list.allowsHost(host); got != want {
			t.Errorf("allowsHost(%q) = %v, want %v", host, got, want)
		}
	}
	for ip, want := range map[string]bool{"192.168.1.10": true, "192.168.1.11": false, "10.1.2.3": true} {
		if got := list.allowsIP(net.ParseIP(ip)); got != want {
			t.Errorf("allowsIP(%s) = %v, want %v", ip, got, want)
		}
	}
}

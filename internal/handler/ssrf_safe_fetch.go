package handler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var errSSRFBlocked = errors.New("目标 URL 指向内网或保留地址，已拒绝")

const maxFetchBodyBytes = 10 << 20

var ssrfBlockedNetworks = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.168.0.0/16",
		"198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4", "::1/128",
		"fc00::/7", "fe80::/10", "ff00::/8",
	}
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		if _, network, err := net.ParseCIDR(cidr); err == nil {
			networks = append(networks, network)
		}
	}
	return networks
}()

func isBlockedFetchIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, network := range ssrfBlockedNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// fetchAllowList 是管理员配置的内网拉取白名单（system_settings.ssrf_allowed_hosts），
// 用于放行部署在内网的 sub-store 等外部订阅源。命中的域名或 IP 跳过内网地址拦截。
var fetchAllowList atomic.Pointer[ssrfAllowList]

const SSRFAllowListSettingKey = "ssrf_allowed_hosts"

type ssrfAllowList struct {
	hosts    []string // 小写域名，同时匹配其子域名
	networks []*net.IPNet
}

// parseSSRFAllowList 解析白名单，每行（或逗号分隔）一项：域名、IP 或 CIDR。
// 返回无法识别的项，便于接口提示。
func parseSSRFAllowList(raw string) (*ssrfAllowList, []string) {
	list := &ssrfAllowList{}
	var invalid []string
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' || r == '\r' }) {
		item = strings.TrimSpace(item)
		if item == "" || strings.HasPrefix(item, "#") {
			continue
		}
		if _, network, err := net.ParseCIDR(item); err == nil {
			list.networks = append(list.networks, network)
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			bits := 128
			if ip.To4() != nil {
				ip, bits = ip.To4(), 32
			}
			list.networks = append(list.networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		host := strings.TrimPrefix(strings.ToLower(item), "*.")
		host = strings.TrimSuffix(host, ".")
		if host == "" || strings.ContainsAny(host, "/:@ ") {
			invalid = append(invalid, item)
			continue
		}
		list.hosts = append(list.hosts, host)
	}
	return list, invalid
}

// SetSSRFAllowList 更新内网拉取白名单，立即生效。
func SetSSRFAllowList(raw string) {
	list, _ := parseSSRFAllowList(raw)
	fetchAllowList.Store(list)
}

func (l *ssrfAllowList) allowsHost(host string) bool {
	if l == nil {
		return false
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, allowed := range l.hosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func (l *ssrfAllowList) allowsIP(ip net.IP) bool {
	if l == nil {
		return false
	}
	for _, network := range l.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func validateFetchURL(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return errors.New("无效的 URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("只允许 http/https 协议的订阅 URL")
	}
	if u.Hostname() == "" {
		return errors.New("URL 缺少主机名")
	}
	return nil
}

func ssrfSafeDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		allow := fetchAllowList.Load()
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedFetchIP(ip) && !allow.allowsIP(ip) {
				return nil, errSSRFBlocked
			}
			return dialer.DialContext(ctx, network, addr)
		}
		if allow.allowsHost(host) {
			return dialer.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("域名 %s 未解析到任何地址", host)
		}
		for _, resolved := range ips {
			if isBlockedFetchIP(resolved.IP) && !allow.allowsIP(resolved.IP) {
				return nil, errSSRFBlocked
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

func newSSRFSafeHTTPClient(timeout time.Duration) *http.Client {
	return newSSRFSafeHTTPClientTLS(timeout, false)
}

// newSSRFSafeHTTPClientTLS 同上,但允许可选地跳过 TLS 证书校验(仅用于拉取订阅内容的场景,
// 由调用方显式传 true)。SSRF 拨号防护始终生效,不受 insecureSkipVerify 影响。
func newSSRFSafeHTTPClientTLS(timeout time.Duration, insecureSkipVerify bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext:           ssrfSafeDialContext(dialer),
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	}
	if insecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("重定向次数过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("重定向到非 http(s) 协议，已拒绝")
			}
			return nil
		},
		Transport: transport,
	}
}

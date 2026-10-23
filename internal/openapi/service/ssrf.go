// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateCIDRs 内网/保留地址段，webhook URL 禁止指向（防 SSRF）。
var privateCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"127.0.0.0/8", "169.254.0.0/16", "100.64.0.0/10",
		"0.0.0.0/8",
		"::1/128", "fc00::/7", "fe80::/10",
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

func isPrivateIP(ip net.IP) bool {
	for _, n := range privateCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// IsPublicIP P1.5 H4：判断 IP 是否为公网地址（先归一化 IPv4-mapped IPv6，
// 再按 ValidatePublicURL 同一套内网段判定）。供 webhook 投递拨号期校验复用，
// 消除 DNS 重绑定 TOCTOU（审计 M28 合并治理）。
func IsPublicIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return !isPrivateIP(ip)
}

// ValidatePublicURL 校验 webhook 回调地址：仅 http/https、禁止内网地址。
// 创建与投递时都调用（投递时再解析一次防 DNS 重绑定）。
func ValidatePublicURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("无效的 URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https scheme")
	}
	if u.User != nil {
		return fmt.Errorf("URL 不允许携带用户信息")
	}

	host := u.Hostname()
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".internal") {
		return fmt.Errorf("不允许内网主机名")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("不允许内网 IP 地址")
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("域名无法解析")
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("域名解析到内网地址")
		}
	}
	return nil
}

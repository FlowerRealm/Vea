// 提供最小 SOCKS5 DialContext，供后端在不依赖系统代理的情况下通过本地入站代理发起 HTTP 请求。
package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// SOCKS5Auth 是 SOCKS5 用户名密码认证信息。
type SOCKS5Auth struct {
	Username string
	Password string
}

// DialSOCKS5Context 通过 SOCKS5 代理建立到 address 的连接（支持 noauth 与 username+password）。
func DialSOCKS5Context(ctx context.Context, proxyAddr string, auth *SOCKS5Auth, network, address string) (net.Conn, error) {
	if strings.TrimSpace(proxyAddr) == "" {
		return nil, errors.New("socks5: proxy addr is empty")
	}
	if network != "" && network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("socks5: unsupported network: %s", network)
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("socks5: split host port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("socks5: invalid port: %w", err)
	}

	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("socks5: dial proxy: %w", err)
	}

	deadline := time.Time{}
	if dl, ok := ctx.Deadline(); ok {
		deadline = dl
		_ = conn.SetDeadline(dl)
	}

	if err := socks5Handshake(conn, auth); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := socks5Connect(conn, host, port); err != nil {
		_ = conn.Close()
		return nil, err
	}

	// 清理握手阶段设置的 deadline，避免连接复用时“用过期 deadline 立即报错”。
	if !deadline.IsZero() {
		_ = conn.SetDeadline(time.Time{})
	}

	return conn, nil
}

func socks5Handshake(conn net.Conn, auth *SOCKS5Auth) error {
	if conn == nil {
		return errors.New("socks5: conn is nil")
	}

	// 认证方法协商
	methods := []byte{0x00} // no auth
	user := strings.TrimSpace(authUsername(auth))
	pass := authPassword(auth)
	if user != "" {
		methods = append(methods, 0x02) // username/password
	}

	req := []byte{0x05, byte(len(methods))}
	req = append(req, methods...)
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5: write hello: %w", err)
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("socks5: read hello resp: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("socks5: invalid ver: %d", resp[0])
	}

	switch resp[1] {
	case 0x00:
		return nil
	case 0x02:
		if user == "" {
			return errors.New("socks5: server requested username/password but auth is empty")
		}
		if len(user) > 255 || len(pass) > 255 {
			return errors.New("socks5: username/password too long")
		}
		creds := []byte{0x01, byte(len(user))}
		creds = append(creds, []byte(user)...)
		creds = append(creds, byte(len(pass)))
		creds = append(creds, []byte(pass)...)
		if _, err := conn.Write(creds); err != nil {
			return fmt.Errorf("socks5: write auth: %w", err)
		}
		ar := make([]byte, 2)
		if _, err := io.ReadFull(conn, ar); err != nil {
			return fmt.Errorf("socks5: read auth resp: %w", err)
		}
		if ar[0] != 0x01 {
			return fmt.Errorf("socks5: invalid auth ver: %d", ar[0])
		}
		if ar[1] != 0x00 {
			return errors.New("socks5: auth rejected")
		}
		return nil
	case 0xff:
		return errors.New("socks5: no acceptable auth method")
	default:
		return fmt.Errorf("socks5: unsupported auth method: %d", resp[1])
	}
}

func socks5Connect(conn net.Conn, host string, port int) error {
	if conn == nil {
		return errors.New("socks5: conn is nil")
	}
	req, err := buildSOCKS5ConnectRequest(host, port)
	if err != nil {
		return fmt.Errorf("socks5: build connect req: %w", err)
	}
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5: write connect req: %w", err)
	}

	respHdr := make([]byte, 4)
	if _, err := io.ReadFull(conn, respHdr); err != nil {
		return fmt.Errorf("socks5: read connect resp: %w", err)
	}
	if respHdr[0] != 0x05 {
		return fmt.Errorf("socks5: invalid ver: %d", respHdr[0])
	}
	if respHdr[1] != 0x00 {
		return fmt.Errorf("socks5: connect error: %d", respHdr[1])
	}

	// 跳过 BND.ADDR + BND.PORT
	switch respHdr[3] {
	case 0x01: // IPv4
		skip := make([]byte, 4+2)
		if _, err := io.ReadFull(conn, skip); err != nil {
			return fmt.Errorf("socks5: read bnd ipv4: %w", err)
		}
	case 0x03: // Domain
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return fmt.Errorf("socks5: read bnd domain len: %w", err)
		}
		skip := make([]byte, int(lenBuf[0])+2)
		if _, err := io.ReadFull(conn, skip); err != nil {
			return fmt.Errorf("socks5: read bnd domain: %w", err)
		}
	case 0x04: // IPv6
		skip := make([]byte, 16+2)
		if _, err := io.ReadFull(conn, skip); err != nil {
			return fmt.Errorf("socks5: read bnd ipv6: %w", err)
		}
	default:
		return fmt.Errorf("socks5: invalid atyp: %d", respHdr[3])
	}

	return nil
}

func buildSOCKS5ConnectRequest(host string, port int) ([]byte, error) {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") && len(host) > 2 {
		host = host[1 : len(host)-1]
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", port)
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req := []byte{0x05, 0x01, 0x00, 0x01}
			req = append(req, ip4...)
			req = append(req, byte(port>>8), byte(port&0xff))
			return req, nil
		}
		ip16 := ip.To16()
		if ip16 == nil {
			return nil, fmt.Errorf("invalid ip: %s", host)
		}
		req := []byte{0x05, 0x01, 0x00, 0x04}
		req = append(req, ip16...)
		req = append(req, byte(port>>8), byte(port&0xff))
		return req, nil
	}

	if len(host) > 255 {
		return nil, fmt.Errorf("host too long: %d", len(host))
	}
	hostBytes := []byte(host)
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(hostBytes))}
	req = append(req, hostBytes...)
	req = append(req, byte(port>>8), byte(port&0xff))
	return req, nil
}

func authUsername(auth *SOCKS5Auth) string {
	if auth == nil {
		return ""
	}
	return auth.Username
}

func authPassword(auth *SOCKS5Auth) string {
	if auth == nil {
		return ""
	}
	return auth.Password
}

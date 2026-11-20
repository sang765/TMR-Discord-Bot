package sources

// Toi thieu SOCKS5 client (RFC 1928 + RFC 1929) cho proxy WARP / wireproxy.
// Khong dung thu vien ngoai.

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	socks5Version  = 0x05
	socks5NoAuth   = 0x00
	socks5UserPass = 0x02
	socks5Connect  = 0x01
	socks5ATypFQDN = 0x03
	socks5ATypIPv4 = 0x01
	socks5ATypIPv6 = 0x04
)

func dialSocks5(ctx context.Context, proxyHost, user, pass, target string) (net.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyHost)
	if err != nil {
		return nil, fmt.Errorf("socks5 connect %s: %w", proxyHost, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	fail := func(e error) (net.Conn, error) {
		conn.Close()
		return nil, e
	}

	methods := []byte{socks5NoAuth}
	if user != "" {
		methods = append(methods, socks5UserPass)
	}
	if _, err := conn.Write(append([]byte{socks5Version, byte(len(methods))}, methods...)); err != nil {
		return fail(err)
	}
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return fail(err)
	}
	if hdr[0] != socks5Version {
		return fail(fmt.Errorf("socks5: version la %d", hdr[0]))
	}
	switch hdr[1] {
	case socks5NoAuth:
	case socks5UserPass:
		u, p := []byte(user), []byte(pass)
		if len(u) > 255 || len(p) > 255 {
			return fail(fmt.Errorf("socks5: user/pass qua dai"))
		}
		msg := append([]byte{1, byte(len(u))}, u...)
		msg = append(msg, byte(len(p)))
		msg = append(msg, p...)
		if _, err := conn.Write(msg); err != nil {
			return fail(err)
		}
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return fail(err)
		}
		if hdr[1] != 0 {
			return fail(fmt.Errorf("socks5: xac thuc that bai"))
		}
	default:
		return fail(fmt.Errorf("socks5: method 0x%02x khong ho tro", hdr[1]))
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return fail(fmt.Errorf("socks5: target sai %q: %w", target, err))
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fail(fmt.Errorf("socks5: port sai %q", portStr))
	}
	if len(host) > 255 {
		return fail(fmt.Errorf("socks5: host qua dai"))
	}
	req := []byte{socks5Version, socks5Connect, 0x00, socks5ATypFQDN, byte(len(host))}
	req = append(req, host...)
	req = append(req, byte(port>>8), byte(port&0xff))
	if _, err := conn.Write(req); err != nil {
		return fail(err)
	}

	rep := make([]byte, 4)
	if _, err := io.ReadFull(conn, rep); err != nil {
		return fail(err)
	}
	if rep[0] != socks5Version || rep[1] != 0 {
		return fail(fmt.Errorf("socks5: server tu choi (code %d)", rep[1]))
	}
	var skip int
	switch rep[3] {
	case socks5ATypIPv4:
		skip = 4 + 2
	case socks5ATypIPv6:
		skip = 16 + 2
	case socks5ATypFQDN:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return fail(err)
		}
		skip = int(n[0]) + 2
	default:
		return fail(fmt.Errorf("socks5: ATYP la %d", rep[3]))
	}
	if _, err := io.ReadFull(conn, make([]byte, skip)); err != nil {
		return fail(err)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

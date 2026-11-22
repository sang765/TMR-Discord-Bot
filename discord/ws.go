package discord

// Minimal RFC 6455 websocket client — đủ cho Discord Gateway (text frames,
// ping/pong, close). Không thêm dependency ngoài stdlib.

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WSConn là kết nối websocket tối giản.
type WSConn struct {
	conn   net.Conn
	br     *bufio.Reader
	closed bool
}

// DialWS kết nối wss:// tới url (chỉ hỗ trợ wss).
func DialWS(rawURL string, timeout time.Duration) (*WSConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", host, &tls.Config{ServerName: u.Hostname()})
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		conn.Close()
		return nil, err
	}
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\n"+
		"Sec-WebSocket-Version: 13\r\n\r\n",
		path, u.Host, base64.StdEncoding.EncodeToString(key))
	if _, err := io.WriteString(conn, req); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	// đọc response headers
	line, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(line, "101") {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake: %s", strings.TrimSpace(line))
	}
	for {
		l, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if l == "\r\n" || l == "\n" {
			break
		}
	}
	return &WSConn{conn: conn, br: br}, nil
}

// SendText gửi một text frame (client phải mask).
func (w *WSConn) SendText(payload []byte) error {
	if w.closed {
		return fmt.Errorf("ws closed")
	}
	n := len(payload)
	header := []byte{0x81}
	switch {
	case n < 126:
		header = append(header, byte(0x80|n))
	case n < 65536:
		header = append(header, 0x80|126, byte(n>>8), byte(n))
	default:
		header = append(header, 0x80|127)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(n))
		header = append(header, b[:]...)
	}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	header = append(header, mask...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := w.conn.Write(append(header, masked...)); err != nil {
		return err
	}
	return nil
}

// ReadText đọc frame tiếp theo, trả payload text. Ping -> Pong tự động,
// binary/close được xử lý và trả error/không có text.
func (w *WSConn) ReadText() ([]byte, error) {
	if w.closed {
		return nil, fmt.Errorf("ws closed")
	}
	for {
		b1, err := w.br.ReadByte()
		if err != nil {
			return nil, err
		}
		fin := b1&0x80 != 0
		opcode := b1 & 0x0f
		b2, err := w.br.ReadByte()
		if err != nil {
			return nil, err
		}
		masked := b2&0x80 != 0
		length := int(b2 & 0x7f)
		if length == 126 {
			var b [2]byte
			if _, err := io.ReadFull(w.br, b[:]); err != nil {
				return nil, err
			}
			length = int(binary.BigEndian.Uint16(b[:]))
		} else if length == 127 {
			var b [8]byte
			if _, err := io.ReadFull(w.br, b[:]); err != nil {
				return nil, err
			}
			length = int(binary.BigEndian.Uint64(b[:]))
		}
		var mask []byte
		if masked {
			mask = make([]byte, 4)
			if _, err := io.ReadFull(w.br, mask); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(w.br, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch opcode {
		case 0x8: // close
			w.closed = true
			return nil, fmt.Errorf("ws close frame")
		case 0x9: // ping -> pong
			if err := w.SendPong(payload); err != nil {
				return nil, err
			}
			continue
		case 0xA: // pong
			continue
		case 0x1, 0x0: // text / continuation
			if !fin {
				return nil, fmt.Errorf("fragmented frame unsupported")
			}
			return payload, nil
		default:
			continue
		}
	}
}

// SendPong gửi pong frame (không mask từ client? RFC: client phải mask — làm theo).
func (w *WSConn) SendPong(payload []byte) error {
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	header := []byte{0x8A, byte(0x80 | len(payload))}
	header = append(header, mask...)
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := w.conn.Write(append(header, masked...))
	return err
}

func (w *WSConn) Close() error {
	w.closed = true
	return w.conn.Close()
}

// SetDeadline đặt read deadline cho heartbeat.
func (w *WSConn) SetDeadline(t time.Time) { _ = w.conn.SetReadDeadline(t) }

// GatewayEvent là payload dispatch từ Discord.
type GatewayEvent struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  int             `json:"s"`
	T  string          `json:"t"`
}

// IdentifyPayload dựng payload IDENTIFY (intents = 0 → KHÔNG cần Message Content Intent;
// vẫn nhận author + timestamp của MESSAGE_CREATE).
func IdentifyPayload(token string, intents int) []byte {
	v := map[string]any{
		"op": 2,
		"d": map[string]any{
			"token":   token,
			"intents": intents,
			"properties": map[string]string{
				"$os":      "linux",
				"$browser": "tmr-bot",
				"$device":  "tmr-bot",
			},
		},
	}
	b, _ := json.Marshal(v)
	return b
}

// ResumePayload dựng RESUME sau reconnect.
func ResumePayload(token, session string, seq int) []byte {
	v := map[string]any{
		"op": 6,
		"d":  map[string]any{"token": token, "session_id": session, "seq": seq},
	}
	b, _ := json.Marshal(v)
	return b
}

// ParseIntents nhận chuỗi số hoặc hex từ env.
func ParseIntents(s string) int {
	if s == "" {
		return 0
	}
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		n, err := strconv.ParseInt(s[2:], 16, 32)
		if err == nil {
			return int(n)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

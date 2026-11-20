package main

// Binary Go thuần chạy trực tiếp trên Android (Termux) không dùng được cấu hình
// hệ thống như trên Linux thường:
//   - DNS: không có /etc/resolv.conf nên net.DefaultResolver rơi về
//     127.0.0.1/[::1]:53 và bị "connection refused".
//   - TLS: các đường CA mặc định của Linux không tồn tại; store /system của
//     Android (nếu đọc được) thường thiếu root hiện đại (Cloudflare, Let's
//     Encrypt) trên máy cũ.
//
// File này phát hiện Android qua /system/build.prop và tự vá cả hai, KHÔNG
// can thiệp gì khi chạy trong proot/proot-distro hay Linux thường.

import (
	"context"
	"net"
	"os"
	"strings"
	"time"
)

// androidDetected: /system/build.prop chỉ tồn tại trên Android.
func androidDetected() bool {
	_, err := os.Stat("/system/build.prop")
	return err == nil
}

// unixCertBundleExists: đang có store CA kiểu Linux (proot/proot-distro/PC)
// — cứ để Go tự xử lý theo mặc định.
func unixCertBundleExists() bool {
	for _, p := range []string{
		"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu
		"/etc/ssl/cert.pem",                                 // Alpine
		"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// termuxCertFile trả về bundle CA của Termux (pkg ca-certificates) nếu có.
func termuxCertFile() string {
	candidates := []string{}
	if p := os.Getenv("PREFIX"); p != "" {
		candidates = append(candidates, p+"/etc/tls/cert.pem")
	}
	candidates = append(candidates, "/data/data/com.termux/files/usr/etc/tls/cert.pem")
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// androidCertDirs: store CA của Android (cả hai đường nếu đọc được, có file).
func androidCertDirs() string {
	var dirs []string
	for _, p := range []string{
		"/apex/com.android.conscrypt/cacerts", // Android 14+
		"/system/etc/security/cacerts",        // Android cũ
	} {
		if ents, err := os.ReadDir(p); err == nil && len(ents) > 0 {
			dirs = append(dirs, p)
		}
	}
	return strings.Join(dirs, ":")
}

func init() {
	if !androidDetected() {
		return
	}

	// --- DNS: hỏi thẳng public DNS khi không có resolv.conf ---
	if _, err := os.Stat("/etc/resolv.conf"); err != nil {
		d := &net.Dialer{Timeout: 4 * time.Second}
		servers := []string{"1.1.1.1:53", "8.8.8.8:53", "9.9.9.9:53"}
		net.DefaultResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var lastErr error
				for _, s := range servers {
					c, err := d.DialContext(ctx, network, s)
					if err == nil {
						return c, nil
					}
					lastErr = err
				}
				return nil, lastErr
			},
		}
	}

	// --- TLS roots: bundle Termux trước (mới nhất), rồi tới store /system ---
	if unixCertBundleExists() {
		return
	}
	if f := termuxCertFile(); f != "" {
		os.Setenv("SSL_CERT_FILE", f)
		return
	}
	if d := androidCertDirs(); d != "" {
		os.Setenv("SSL_CERT_DIR", d)
	}
}

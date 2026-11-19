package sources

import (
	"io"
	"net/http"
	"strings"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// newRequest builds a request with browser-like headers
// (một số site dùng Cloudflare, UA lạ + thiếu header là 403 ngay).
func newRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	return req, nil
}

// doRequest performs req, returns body (max 4MB) and status code.
func doRequest(req *http.Request) (string, int, error) {
	resp, err := HTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(b), resp.StatusCode, nil
}

// JinaGet fetches a page through r.jina.ai (reader proxy, render cả JS).
// format: "html" hoặc "markdown". Dùng khi direct bị Cloudflare chặn (403).
func JinaGet(target, format string) (string, error) {
	req, err := http.NewRequest("GET", "https://r.jina.ai/"+target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("X-Return-Format", format)
	req.Header.Set("X-No-Cache", "true")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")

	body, code, err := doRequest(req)
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmtErr("jina HTTP %d cho %s", code, target)
	}
	if strings.EqualFold(format, "markdown") {
		if i := strings.Index(body, "Markdown Content:"); i >= 0 {
			body = body[i+len("Markdown Content:"):]
		}
	}
	return body, nil
}

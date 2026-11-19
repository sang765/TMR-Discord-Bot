package sources

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// apiUA là UA "thẳng thắn" cho API endpoint: MangaDex tự chặn request
// giả danh browser trên API ("if this isn't a browser, don't send a browser UA").
const apiUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36 TMRBot/1.0"

// newRequest builds a plain API-style request (chỉ UA, không Sec-Fetch).
func newRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", apiUA)
	return req, nil
}

// newBrowserRequest builds a browser-like request cho trang HTML
// (một số site dùng Cloudflare, thiếu header browser là 403).
func newBrowserRequest(method, url string) (*http.Request, error) {
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

// ---- r.jina.ai (reader proxy) với throttle: free tier ~20 req/phút ----

var (
	jinaMu   sync.Mutex
	jinaLast time.Time
)

const jinaGap = 3500 * time.Millisecond

var challengeMarkers = []string{"Just a moment...", "Attention Required", "Browser verification", "cf-chl"}

func looksLikeChallenge(body string) bool {
	for _, m := range challengeMarkers {
		if strings.Contains(body, m) {
			return true
		}
	}
	return false
}

// JinaGet fetches a page through r.jina.ai (render cả JS).
// format: "html" hoặc "markdown". Dùng khi direct bị Cloudflare chặn (403).
func JinaGet(target, format string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		jinaMu.Lock()
		if wait := jinaGap - time.Since(jinaLast); wait > 0 {
			time.Sleep(wait)
		}
		jinaLast = time.Now()
		jinaMu.Unlock()

		req, err := http.NewRequest("GET", "https://r.jina.ai/"+target, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", apiUA)
		req.Header.Set("X-Return-Format", format)
		req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")

		body, code, err := doRequest(req)
		if err != nil {
			return "", err
		}
		if code != 200 {
			lastErr = fmtErr("jina HTTP %d cho %s", code, target)
			time.Sleep(2 * time.Second)
			continue
		}
		if looksLikeChallenge(body) && attempt == 0 {
			lastErr = fmtErr("jina bị Cloudflare challenge: %s", target)
			time.Sleep(2 * time.Second)
			continue
		}
		if strings.EqualFold(format, "markdown") {
			if i := strings.Index(body, "Markdown Content:"); i >= 0 {
				body = body[i+len("Markdown Content:"):]
			}
		}
		return body, nil
	}
	return "", lastErr
}

package sources

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Timezone the sites use (Vietnam).
var tzVN = time.FixedZone("+07:00", 7*3600)

var relRe = regexp.MustCompile(`(\d+|\w+)\s*(giây|phút|giờ|ngày|tuần|tháng|năm)\s*trước`)

var unitMap = map[string]time.Duration{
	"giây":  time.Second,
	"phút":  time.Minute,
	"giờ":   time.Hour,
	"ngày":  24 * time.Hour,
	"tuần":  7 * 24 * time.Hour,
	"tháng": 30 * 24 * time.Hour,
	"năm":   365 * 24 * time.Hour,
}

// ParseRelVN parses Vietnamese relative time like "2 ngày trước", "5 giờ trước".
// "vài" is treated as 3.
func ParseRelVN(s string, now time.Time) (time.Time, bool) {
	m := relRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, false
	}
	n := int64(3) // "vài"
	if v, err := strconv.ParseInt(m[1], 10, 64); err == nil {
		n = v
	}
	d, ok := unitMap[m[2]]
	if !ok {
		return time.Time{}, false
	}
	return now.Add(-time.Duration(n) * d), true
}

var dateRe = regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})`)

// ParseDDMMYYYY parses "23/09/2026" in VN timezone.
func ParseDDMMYYYY(s string) (time.Time, bool) {
	m := dateRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	d, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	y, _ := strconv.Atoi(m[3])
	return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, tzVN), true
}

// StripTags removes HTML tags and unescapes entities, collapsing whitespace.
func StripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			b.WriteRune(r)
		}
	}
	out := html.UnescapeString(b.String())
	return strings.Join(strings.Fields(out), " ")
}

// Truncate shortens s to max runes, adding an ellipsis.
func Truncate(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}

// NormTitle normalizes a series title for cross-source dedup.
func NormTitle(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'à' && r <= 'ỹ': // keep vietnamese letters roughly
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ChapterNorm normalizes a chapter label for dedup ("Chương 58" -> "58").
var chapNumRe = regexp.MustCompile(`(\d+(?:\.\d+)?)`)

func ChapterNorm(label string) string {
	m := chapNumRe.FindStringSubmatch(label)
	if m == nil {
		return strings.ToLower(strings.TrimSpace(label))
	}
	return m[1]
}

// AbsoluteURL resolves href against base if it is relative.
func AbsoluteURL(base, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if !strings.HasPrefix(href, "/") {
		href = "/" + href
	}
	return strings.TrimRight(base, "/") + href
}

func metaContent(raw, name string) string {
	re := regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)="` + regexp.QuoteMeta(name) + `"[^>]+content="([^"]*)"`)
	if m := re.FindStringSubmatch(raw); m != nil {
		return html.UnescapeString(m[1])
	}
	// attribute order swapped
	re2 := regexp.MustCompile(`(?i)<meta[^>]+content="([^"]*)"[^>]+(?:property|name)="` + regexp.QuoteMeta(name) + `"`)
	if m := re2.FindStringSubmatch(raw); m != nil {
		return html.UnescapeString(m[1])
	}
	return ""
}

func fmtErr(format string, a ...any) error { return fmt.Errorf(format, a...) }

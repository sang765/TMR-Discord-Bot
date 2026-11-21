package sources

// Probe kiểm tra "sức khoẻ cấu trúc" của từng nguồn: tải đúng trang feed mà
// Fetch dùng, rồi đếm số mục parser hiện tại trích được. Khác với Fetch,
// Probe KHÔNG đọc/ghi SeenStore, KHÔNG sinh item — nên chạy định kỳ sẽ không
// làm bot nhầm "cập nhật mới" hay sửa state.json.
//
// Count == 0 kèm HTTP 200 là dấu hiệu site đổi markup (class/html/json shape),
// cần sửa regex/parser. Các lỗi 403/429/DNS/TLS là lỗi hạ tầng, đừng sửa parser.

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Probe DocLn: trang nhóm phải có ít nhất một card series-name.
func (d *DocLn) Probe() (int, error) {
	url := fmt.Sprintf("%s/nhom-dich/%s-the-mavericks?page=1", d.Base, d.GroupID)
	raw, err := d.get(url) // đã có jina fallback bên trong
	if err != nil {
		return 0, err
	}
	n := len(doclnSeriesRe.FindAllStringIndex(raw, -1))
	if n == 0 {
		return 0, fmtErr("HTTP 200 nhưng 0 card series-name — markup đổi?")
	}
	return n, nil
}

// Probe MangaDex: API v10 phải trả result=ok, có data và relationship manga.
func (m *MangaDex) Probe() (int, error) {
	req, _ := newRequest("GET", m.apiURL())
	body, code, err := doRequest(req)
	if err != nil {
		return 0, err
	}
	if code != 200 {
		return 0, fmtErr("API HTTP %d: %.120s", code, body)
	}
	var resp mdResp
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return 0, fmtErr("JSON không parse được (API shape đổi?): %v", err)
	}
	if resp.Result != "ok" {
		return 0, fmtErr("result=%q", resp.Result)
	}
	if len(resp.Data) == 0 {
		return 0, fmtErr("API trả về 0 chapter")
	}
	hasManga := false
	for _, ch := range resp.Data {
		for _, rel := range ch.Relationships {
			if rel.Type == "manga" && rel.ID != "" {
				hasManga = true
			}
		}
	}
	if !hasManga {
		return 0, fmtErr("không có relationship manga — API shape đổi?")
	}
	return len(resp.Data), nil
}

// Probe Mòe Truyện: panel "Hoạt động gần đây" phải có team-update-item.
func (m *Moe) Probe() (int, error) {
	url := strings.TrimRight(m.Base, "/") + m.TeamPath
	req, err := newBrowserRequest("GET", url)
	if err != nil {
		return 0, err
	}
	body, code, err := doRequest(req)
	parsed := []moeItem{}
	if err == nil && code == 200 {
		parsed = m.parse(body)
	}
	if len(parsed) == 0 {
		if jb, jerr := JinaGet(url, "html"); jerr == nil {
			parsed = m.parse(jb)
		}
	}
	if len(parsed) == 0 {
		if err != nil {
			return 0, fmtErr("direct: %v (jina cũng lỗi)", err)
		}
		return 0, fmtErr("HTTP %d nhưng 0 team-update-item — markup đổi?", code)
	}
	return len(parsed), nil
}

// Probe Cứu Truyện: trang new-mangas (direct HTML hoặc jina markdown) phải parse được mục.
func (c *CuuTruyen) Probe() (int, error) {
	body, isMD, err := c.fetchPage()
	if err != nil {
		return 0, err
	}
	var parsed []ctItem
	if isMD {
		parsed = c.parseMarkdown(body)
	} else {
		parsed = c.parseHTML(body)
	}
	if len(parsed) == 0 {
		return 0, fmtErr("0 mục cập nhật (markdown/HTML shape đổi?)")
	}
	return len(parsed), nil
}

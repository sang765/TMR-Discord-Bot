package sources

import (
	"strings"
)

// TeamName trả tên nhóm dịch từ config ID/path, để footer card hiện đúng nhóm
// thay vì hardcode "The Mavericks". Các nhóm nếu không có trong map thì tự
// derive từ slug (vd /team/74/exist-in-this-world → "Exist In This World").
func TeamName(id string) string {
	known := map[string]string{
		"3474":                                 "The Mavericks",
		"+695":                                 "The Mavericks",
		"256":                                  "The Mavericks",
		"1a653584-126f-4775-9af8-e06e250ad829": "The Mavericks",
		"9759e8b8-c76a-4d1e-b672-c88e9d5f67e6": "Exist In This World",
		"74":                                   "Exist In This World",
	}
	if v, ok := known[id]; ok {
		return v
	}
	// /team/74/exist-in-this-world → lấy phần slug cuối, title-case
	if i := strings.LastIndex(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	if id == "" {
		return ""
	}
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

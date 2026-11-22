// Command v2test tạo payload Components V2 ra stdout (JSON), để curl gửi từ
// Termux — tránh lỗi DNS của binary Go tĩnh chạy ngoài app chính.
//
//   go run ./cmd/v2test > payload.json
//   scp payload.json termux:~/discord-bot/
//   curl -X POST -H "Authorization: Bot $TOKEN" -H "Content-Type: application/json" --data @payload.json URL
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"tmr-bot/discord"
)

func main() {
	const role = "1424440045116330204"
	const cover = "https://u.truyen.moe/uploads/covers/2859-khi-de-tu-cua-phap-su-mim-cuoi-md.webp"

	inner := []discord.Component{
		{Type: discord.CompTextDisplay, Content: discord.MentionRole(role) + "\n# TEST Components V2 (tự xoá)\nĐã upload **Chương 0 — thử giao diện**"},
		{Type: discord.CompMediaGallery, Items: []discord.MediaItem{{Media: discord.MediaSource{URL: cover}}}},
		{Type: discord.CompActionRow, Components: []discord.Component{{
			Type: discord.CompButton, Style: discord.ButtonStyleLink,
			Label: "Đọc tại Mòe Truyện", URL: "https://truyen.moe/team/256/the-mavericks",
		}}},
		{Type: discord.CompTextDisplay, Content: "-# The Mavericks • Mòe Truyện • " + time.Now().Format("02/01 15:04")},
	}
	msg := discord.Message{
		AllowedMentions: &discord.AllowedMentions{Parse: []string{}, Roles: []string{role}},
		Flags:      discord.FlagComponentsV2,
		Components: []discord.Component{{Type: discord.CompContainer, AccentColor: 0xF39C12, Components: inner}},
	}
	b, _ := json.Marshal(msg)
	fmt.Println(string(b))
}

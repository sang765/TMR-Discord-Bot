// Command v2test gửi một tin Components V2 test lên kênh status rồi xoá ngay.
// Dùng để validate payload V2 với Discord API. Chạy: go run ./cmd/v2test
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"tmr-bot/discord"
)

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if os.Getenv(strings.TrimSpace(k)) == "" {
			os.Setenv(strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
}

func main() {
	loadDotEnv("../.env")
	loadDotEnv(".env")
	dc := discord.New(os.Getenv("DISCORD_TOKEN"))
	ch := os.Getenv("STATUS_CHANNEL_ID")
	if ch == "" {
		ch = "1447576235550052455"
	}
	if os.Getenv("DISCORD_TOKEN") == "" {
		fmt.Println("không có DISCORD_TOKEN")
		os.Exit(1)
	}

	inner := []discord.Component{
		{Type: discord.CompTextDisplay, Content: "# TEST Components V2 (tự xoá)\nĐã upload **Chương 0 — thử giao diện**"},
		{Type: discord.CompMediaGallery, Items: []discord.MediaItem{{Media: discord.MediaSource{
			URL: "https://i2.hako.vip/ln/series/covers/s28770-217241bc-a084-48ea-8961-4c507b8c974b.webp"}}}},
		{Type: discord.CompActionRow, Components: []discord.Component{{
			Type: discord.CompButton, Style: discord.ButtonStyleLink,
			Label: "Đọc tại Mòe Truyện", URL: "https://truyen.moe/team/256/the-mavericks",
		}}},
		{Type: discord.CompTextDisplay, Content: "-# The Mavericks • Mòe Truyện • " + time.Now().Format("02/01 15:04")},
	}
	msg := discord.Message{
		Flags:      discord.FlagComponentsV2,
		Components: []discord.Component{{Type: discord.CompContainer, AccentColor: 0xF39C12, Components: inner}},
	}

	id, err := dc.SendMessage(ch, msg)
	if err != nil {
		fmt.Println("SEND_FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("SEND_OK id=", id)
	if err := dc.DeleteMessage(ch, id); err != nil {
		fmt.Println("DELETE_FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("DELETE_OK — payload V2 được Discord chấp nhận")
}

// Command preview dựng payload mẫu cho card "truyện mới" theo mẫu mới của Sáng,
// in JSON ra stdout để curl gửi thử. Không ping role (allowed_mentions.parse=[]).
// Chạy: go run ./cmd/preview
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tmr-bot/discord"
)

func preview(source, title, desc, url, cover string, color int, genres, collabs []string) discord.Message {
	src := map[string]string{"docln": "DocLn", "mangadex": "MangaDex", "moe": "Mòe Truyện", "cuutruyen": "Cứu Truyện"}[source]
	text := "# Truyện mới đã cập bến rồi đây\n" +
		"**🏷️ Tên:** " + title + "\n" +
		"**📃 Mô tả:**\n" + desc
	if len(genres) > 0 {
		tags := make([]string, 0, len(genres))
		for _, g := range genres {
			tags = append(tags, "`"+g+"`")
		}
		text += "\n**📋 Thể loại:**\n" + strings.Join(tags, ", ")
	}
	if len(collabs) > 0 {
		text += "\n\n**🤝 Collab với:**\n"
		for _, c := range collabs {
			name, link, _ := strings.Cut(c, "|")
			text += "- **[" + name + "](" + link + ")**\n"
		}
	}
	text = text + "\n<@&1424440045116330204>"

	inner := []discord.Component{
		{Type: discord.CompTextDisplay, Content: text},
		{Type: discord.CompMediaGallery, Items: []discord.MediaItem{{Media: discord.MediaSource{URL: cover}}}},
		{Type: discord.CompActionRow, Components: []discord.Component{{
			Type: discord.CompButton, Style: discord.ButtonStyleLink,
			Label: "Đọc tại " + src, URL: url,
		}}},
		{Type: discord.CompTextDisplay, Content: "-# The Mavericks | " + src + " | " + time.Now().In(time.FixedZone("+07", 7*3600)).Format("02/01 15:04")},
	}
	return discord.Message{
		Flags:           discord.FlagComponentsV2,
		AllowedMentions: &discord.AllowedMentions{Parse: []string{}},
		Components:      []discord.Component{{Type: discord.CompContainer, AccentColor: color, Components: inner}},
	}
}

func main() {
	const cover = "https://u.truyen.moe/uploads/covers/2808-digimon-eggs-md.webp?t=2986a8b0ff2cc479db220d70d1efbe2446adeac16ed0835c06b8b50000aa19e9"
	items := []struct {
		name, source, title, desc, url string
		color                          int
		genres, collabs                []string
	}{
		{"NOVEL MỚI", "docln", "Ví Dụ Novel Mới — Bản Xem Thử", "Một bộ light novel về những cuộc gặp gỡ bất ngờ và hành trình tìm lại ký ức bị lãng quên.", "https://docln.sbs/truyen/9999-example-novel", 0x7C3AED, []string{"Romance", "School", "Comedy"}, nil},
		{"MANGA MỚI + COLLAB", "moe", "Digimon Eggs", "Cuộc phiêu lưu của những quả trứng Digimon bí ẩn tại thế giới kỹ thuật số.", "https://truyen.moe/manga/2808-digimon-eggs", 0xF39C12, []string{"Action", "Adventure", "Fantasy"}, []string{"Tạp kỹ Helikopter|https://truyen.moe/team/117/tap-ky-helikopter"}},
	}
	for _, it := range items {
		b, _ := json.Marshal(preview(it.source, it.title, it.desc, it.url, cover, it.color, it.genres, it.collabs))
		fmt.Println("### " + it.name)
		fmt.Println(string(b))
	}
}

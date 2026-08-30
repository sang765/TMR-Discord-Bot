package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"TMR-Discord-Bot/config"
	"TMR-Discord-Bot/utils"

	"github.com/bwmarrin/discordgo"
)

func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, kc *utils.KonachanClient, zc *utils.ZerochanClient, guildID string) {
	if m.Author.Bot {
		return
	}

	content := m.Content
	prefix := cfg.Bot.Prefix

	if !strings.HasPrefix(content, prefix) && !strings.HasPrefix(content, ".") {
		return
	}

	actualPrefix := prefix
	if strings.HasPrefix(content, ".") {
		actualPrefix = "."
	}

	args := strings.Fields(content[len(actualPrefix):])
	if len(args) == 0 {
		return
	}

	cmd := strings.ToLower(args[0])

	// Permission check: only Manage Server or Administrator can use the bot
	if !hasManageServerOrAdmin(s, m.Author.ID, m.GuildID) {
		sendMessage(s, m, "You need **Manage Server** or **Administrator** permission to use this bot.")
		return
	}

	switch cmd {
	case "help":
		sendHelp(s, m, cfg)
	case "seticon":
		handleSetIcon(s, m, kc, zc, cfg, guildID)
	case "setbanner":
		handleSetBanner(s, m, kc, zc, cfg, guildID)
	case "boost":
		handleBoostCheck(s, m, guildID)
	case "config":
		sendConfig(s, m, cfg)
	case "interval":
		if len(args) > 1 {
			handleSetInterval(s, m, cfg, args[1])
		} else {
			sendMessage(s, m, fmt.Sprintf("Current interval: %d seconds", cfg.Auto.Interval))
		}
	case "toggle":
		if len(args) > 1 {
			handleToggle(s, m, cfg, args[1])
		} else {
			sendMessage(s, m, "Usage: !toggle <icon|banner>")
		}
	case "toggleautoicon":
		cfg.Auto.IconEnabled = !cfg.Auto.IconEnabled
		config.SaveConfig(cfg)
		sendMessage(s, m, fmt.Sprintf("Auto icon: %v", cfg.Auto.IconEnabled))
	case "toggleautobanner":
		cfg.Auto.BannerEnabled = !cfg.Auto.BannerEnabled
		config.SaveConfig(cfg)
		sendMessage(s, m, fmt.Sprintf("Auto banner: %v", cfg.Auto.BannerEnabled))
	case "source":
		if len(args) > 1 {
			handleSetSource(s, m, cfg, args[1])
		} else {
			sendMessage(s, m, fmt.Sprintf("Current source: `%s`\nAvailable: `konachan`, `zerochan`", cfg.Source))
		}
	case "setprefix":
		if len(args) > 1 {
			handleSetPrefix(s, m, cfg, args[1])
		} else {
			sendMessage(s, m, fmt.Sprintf("Current prefix: `%s`", cfg.Bot.Prefix))
		}
	case "setstatus":
		if len(args) > 1 {
			handleSetStatus(s, m, cfg, strings.Join(args[1:], " "))
		} else {
			sendMessage(s, m, fmt.Sprintf("Current status: `%s`", cfg.Bot.Status))
		}
	case "setrating":
		if len(args) > 1 {
			handleSetRating(s, m, cfg, args[1])
		} else {
			sendMessage(s, m, fmt.Sprintf("Current rating: `%s` (s=safe, q=questionable, e=explicit)", cfg.Konachan.Rating))
		}
	case "setscore":
		if len(args) > 1 {
			handleSetScore(s, m, cfg, args[1])
		} else {
			sendMessage(s, m, fmt.Sprintf("Current min score: %d", cfg.Konachan.MinScore))
		}
	default:
		sendMessage(s, m, fmt.Sprintf("Unknown command: %s. Use %shelp for help.", cmd, actualPrefix))
	}
}

func sendHelp(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config) {
	prefix := cfg.Bot.Prefix
	content := fmt.Sprintf("**TMR Bot Commands**\n\n"+
		"**General:**\n"+
		"`%shelp` - Show this help\n"+
		"`%sboost` - Check server boost level\n"+
		"`%sconfig` - Show bot config\n\n"+
		"**Image Commands:**\n"+
		"`%sseticon` - Set server icon from current source\n"+
		"`%ssetbanner` - Set server banner from current source\n\n"+
		"**Settings:**\n"+
		"`%sinterval <seconds>` - Set auto change interval\n"+
		"`%ssetprefix <prefix>` - Set bot prefix\n"+
		"`%ssetstatus <text>` - Set bot status\n"+
		"`%ssetrating <s|q|e>` - Set image rating (konachan)\n"+
		"`%ssetscore <number>` - Set minimum score (konachan)\n"+
		"`%ssource <konachan|zerochan>` - Set image source\n\n"+
		"**Toggle:**\n"+
		"`%stoggle icon` - Toggle auto icon\n"+
		"`%stoggle banner` - Toggle auto banner\n"+
		"`%stoggleautoicon` - Toggle auto icon (alias)\n"+
		"`%stoggleautobanner` - Toggle auto banner (alias)\n\n"+
		"⚠️ Requires **Manage Server** or **Administrator** permission.\n"+
		"Powered by konachan.net & zerochan.net",
		prefix, prefix, prefix, prefix, prefix,
		prefix, prefix, prefix, prefix, prefix, prefix,
		prefix, prefix, prefix, prefix)

	sendMessage(s, m, content)
}

func handleSetIcon(s *discordgo.Session, m *discordgo.MessageCreate, kc *utils.KonachanClient, zc *utils.ZerochanClient, cfg *config.Config, guildID string) {
	sendMessage(s, m, fmt.Sprintf("Fetching random icon from %s...", cfg.Source))

	var imgURL string

	switch cfg.Source {
	case "zerochan":
		entry, e := zc.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL = entry.GetImageURL()
		if imgURL == "" {
			sendMessage(s, m, "Error: No valid image URL found from zerochan")
			return
		}
	default: // konachan
		iconKC := utils.NewKonachanClient(cfg.Konachan.IconTags, cfg.Konachan.Rating, cfg.Konachan.MinScore)
		img, e := iconKC.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL = img.FileURL
	}

	if err := downloadAndSetIconFromURL(s, m, imgURL, guildID); err != nil {
		sendMessage(s, m, fmt.Sprintf("Error setting icon: %v", err))
		return
	}

	sendMessage(s, m, "Icon updated!")
}

func handleSetBanner(s *discordgo.Session, m *discordgo.MessageCreate, kc *utils.KonachanClient, zc *utils.ZerochanClient, cfg *config.Config, guildID string) {
	sendMessage(s, m, fmt.Sprintf("Fetching random banner from %s...", cfg.Source))

	var imgURL string

	switch cfg.Source {
	case "zerochan":
		entry, e := zc.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL = entry.GetImageURL()
		if imgURL == "" {
			sendMessage(s, m, "Error: No valid image URL found from zerochan")
			return
		}
	default: // konachan
		bannerKC := utils.NewKonachanClient(cfg.Konachan.BannerTags, cfg.Konachan.Rating, cfg.Konachan.MinScore)
		img, e := bannerKC.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL = img.FileURL
	}

	if err := downloadAndSetBannerFromURL(s, m, imgURL, guildID); err != nil {
		sendMessage(s, m, fmt.Sprintf("Error setting banner: %v", err))
		return
	}

	sendMessage(s, m, "Banner updated!")
}

func downloadAndSetIcon(s *discordgo.Session, m *discordgo.MessageCreate, img *utils.KonachanPost, guildID string) error {
	data, err := downloadImage(img.FileURL)
	if err != nil {
		return err
	}

	cropped, err := utils.CropBytesToSquare(data, 512)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(cropped))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	return err
}

func downloadAndSetIconFromURL(s *discordgo.Session, m *discordgo.MessageCreate, imgURL string, guildID string) error {
	data, err := downloadImage(imgURL)
	if err != nil {
		return err
	}

	cropped, err := utils.CropBytesToSquare(data, 512)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(cropped))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	return err
}

func downloadAndSetBanner(s *discordgo.Session, m *discordgo.MessageCreate, img *utils.KonachanPost, guildID string) error {
	data, err := downloadImage(img.FileURL)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(data))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Banner: dataURI,
	})
	return err
}

func downloadAndSetBannerFromURL(s *discordgo.Session, m *discordgo.MessageCreate, imgURL string, guildID string) error {
	data, err := downloadImage(imgURL)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(data))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Banner: dataURI,
	})
	return err
}

func downloadImage(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func handleBoostCheck(s *discordgo.Session, m *discordgo.MessageCreate, guildID string) {
	guild, err := s.Guild(guildID)
	if err != nil {
		sendMessage(s, m, fmt.Sprintf("Error fetching guild: %v", err))
		return
	}

	content := fmt.Sprintf("**Server Boost Info**\n\n"+
		"**Boost Level:** %d\n"+
		"**Boost Count:** %d boosts\n"+
		"**Icon Upload:** %s\n"+
		"**Banner Upload:** %s",
		int(guild.PremiumTier),
		guild.PremiumSubscriptionCount,
		getBoostFeature(int(guild.PremiumTier), 1, "Unlocked at Level 1"),
		getBoostFeature(int(guild.PremiumTier), 2, "Unlocked at Level 2"))

	sendMessage(s, m, content)
}

func getBoostFeature(currentTier, requiredTier int, lockedMsg string) string {
	if currentTier >= requiredTier {
		return "Unlocked"
	}
	return lockedMsg
}

func sendConfig(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config) {
	iconStatus := "OFF"
	if cfg.Auto.IconEnabled {
		iconStatus = "ON"
	}
	bannerStatus := "OFF"
	if cfg.Auto.BannerEnabled {
		bannerStatus = "ON"
	}

	content := fmt.Sprintf("**Bot Configuration**\n\n"+
		"**Bot Settings:**\n"+
		"- Prefix: `%s`\n"+
		"- Status: `%s`\n"+
		"- Source: `%s`\n\n"+
		"**Auto Settings:**\n"+
		"- Auto Icon: `%s`\n"+
		"- Auto Banner: `%s`\n"+
		"- Interval: `%d seconds` (%d min)\n\n"+
		"**Konachan Settings:**\n"+
		"- Rating: `%s`\n"+
		"- Min Score: `%d`\n\n"+
		"**Server Info:**\n"+
		"- Guild ID: `%s`",
		cfg.Bot.Prefix,
		cfg.Bot.Status,
		cfg.Source,
		iconStatus,
		bannerStatus,
		cfg.Auto.Interval,
		cfg.Auto.Interval/60,
		cfg.Konachan.Rating,
		cfg.Konachan.MinScore,
		m.GuildID)

	sendMessage(s, m, content)
}

func handleSetInterval(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	var interval int
	if _, err := fmt.Sscanf(value, "%d", &interval); err != nil || interval < 60 {
		sendMessage(s, m, "Interval must be a number >= 60 seconds")
		return
	}
	cfg.Auto.Interval = interval
	config.SaveConfig(cfg)
	sendMessage(s, m, fmt.Sprintf("Interval set to %d seconds", interval))
}

func handleToggle(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, target string) {
	switch strings.ToLower(target) {
	case "icon":
		cfg.Auto.IconEnabled = !cfg.Auto.IconEnabled
		config.SaveConfig(cfg)
		sendMessage(s, m, fmt.Sprintf("Auto icon: %v", cfg.Auto.IconEnabled))
	case "banner":
		cfg.Auto.BannerEnabled = !cfg.Auto.BannerEnabled
		config.SaveConfig(cfg)
		sendMessage(s, m, fmt.Sprintf("Auto banner: %v", cfg.Auto.BannerEnabled))
	default:
		sendMessage(s, m, "Usage: !toggle <icon|banner>")
	}
}

func handleSetPrefix(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	if len(value) > 5 {
		sendMessage(s, m, "Prefix must be 1-5 characters")
		return
	}
	cfg.Bot.Prefix = value
	config.SaveConfig(cfg)
	sendMessage(s, m, fmt.Sprintf("Prefix set to `%s`", value))
}

func handleSetStatus(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	cfg.Bot.Status = value
	config.SaveConfig(cfg)
	sendMessage(s, m, fmt.Sprintf("Status set to `%s`", value))
}

func handleSetRating(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	value = strings.ToLower(value)
	if value != "s" && value != "q" && value != "e" {
		sendMessage(s, m, "Rating must be `s` (safe), `q` (questionable), or `e` (explicit)")
		return
	}
	cfg.Konachan.Rating = value
	config.SaveConfig(cfg)
	sendMessage(s, m, fmt.Sprintf("Rating set to `%s`", value))
}

func handleSetScore(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	var score int
	if _, err := fmt.Sscanf(value, "%d", &score); err != nil || score < 0 {
		sendMessage(s, m, "Score must be a number >= 0")
		return
	}
	cfg.Konachan.MinScore = score
	config.SaveConfig(cfg)
	sendMessage(s, m, fmt.Sprintf("Min score set to `%d`", score))
}

func handleSetSource(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	value = strings.ToLower(value)
	switch value {
	case "konachan", "zerochan":
		cfg.Source = value
		config.SaveConfig(cfg)
		sendMessage(s, m, fmt.Sprintf("Image source set to `%s`", value))
	default:
		sendMessage(s, m, "Available sources: `konachan`, `zerochan`")
	}
}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, content string) {
	s.ChannelMessageSend(m.ChannelID, content)
}

func hasManageServerOrAdmin(s *discordgo.Session, userID, guildID string) bool {
	member, err := s.GuildMember(guildID, userID)
	if err != nil {
		return false
	}

	// Check Administrator first
	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			continue
		}
		if role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
			return true
		}
		if role.Permissions&discordgo.PermissionManageGuild == discordgo.PermissionManageGuild {
			return true
		}
	}
	return false
}

func InteractionHandler(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, kc *utils.KonachanClient, zc *utils.ZerochanClient, guildID string) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// Permission check: only Manage Server or Administrator can use the bot
	if !hasManageServerOrAdmin(s, i.Member.User.ID, i.GuildID) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You need **Manage Server** or **Administrator** permission to use this bot.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	data := i.ApplicationCommandData()

	switch data.Name {
	case "seticon":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Fetching random icon...",
			},
		})
	case "setbanner":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Fetching random banner...",
			},
		})
	case "boost":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Checking boost level...",
			},
		})
	case "help":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "**TMR Bot Slash Commands**\n\n" +
					"`/seticon` - Set random icon\n" +
					"`/setbanner` - Set random banner\n" +
					"`/boost` - Check boost level\n" +
					"`/help` - Show this help",
			},
		})
	}
}

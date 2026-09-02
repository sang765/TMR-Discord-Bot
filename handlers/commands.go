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

// Rate limiter for Discord GuildEdit calls (~2 req/10s safe margin)
var guildEditLimiter = utils.NewDiscordRateLimiter(1 * time.Second)

// RPS monitor for tracking Discord API calls
var rpsMonitor *utils.RPSMonitor

// SetRPSMonitor sets the RPS monitor for this package.
func SetRPSMonitor(m *utils.RPSMonitor) {
	rpsMonitor = m
}

func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, kc *utils.KonachanClient, zc *utils.ZerochanClient, whc *utils.WallhavenClient, mwc *utils.MoeWallsClient, guildID string) {
	if m.Author.Bot {
		return
	}

	// Only process messages from the configured guild
	if m.GuildID != guildID {
		return
	}

	content := m.Content
	prefix := cfg.Bot.Prefix

	if !strings.HasPrefix(content, prefix) {
		return
	}

	actualPrefix := prefix

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
		handleSetIcon(s, m, kc, zc, whc, mwc, cfg, guildID)
	case "setbanner":
		handleSetBanner(s, m, kc, zc, whc, mwc, cfg, guildID)
	case "boost":
		handleBoostCheck(s, m, guildID)
	case "config":
		sendConfig(s, m, cfg)
	case "interval":
		if len(args) > 1 {
			handleSetInterval(s, m, cfg, args[1])
		} else {
			sendMessage(s, m, fmt.Sprintf("Current interval: %s", formatDuration(time.Duration(cfg.Auto.Interval)*time.Second)))
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
			sendMessage(s, m, fmt.Sprintf("Current source: `%s`\nAvailable: `konachan`, `zerochan`, `wallhaven`, `moewalls`", cfg.Source))
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
	case "rps":
		handleRPSCheck(s, m)
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
		"`%sconfig` - Show bot config\n"+
		"`%srps` - Check current Discord API RPS\n\n"+
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
		prefix, prefix, prefix, prefix, prefix)

	sendMessage(s, m, content)
}

func handleSetIcon(s *discordgo.Session, m *discordgo.MessageCreate, kc *utils.KonachanClient, zc *utils.ZerochanClient, whc *utils.WallhavenClient, mwc *utils.MoeWallsClient, cfg *config.Config, guildID string) {
	sendMessage(s, m, fmt.Sprintf("Fetching random icon from %s...", cfg.Source))

	switch cfg.Source {
	case "zerochan":
		entry, e := zc.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL := entry.GetImageURL()
		if imgURL == "" {
			sendMessage(s, m, "Error: No valid image URL found from zerochan")
			return
		}
		if err := downloadAndSetIconFromURL(s, m, imgURL, guildID); err != nil {
			sendMessage(s, m, fmt.Sprintf("Error setting icon: %v", err))
			return
		}
	case "wallhaven":
		entry, e := whc.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL := entry.GetImageURL()
		if imgURL == "" {
			sendMessage(s, m, "Error: No valid image URL found from wallhaven")
			return
		}
		if err := downloadAndSetIconFromURL(s, m, imgURL, guildID); err != nil {
			sendMessage(s, m, fmt.Sprintf("Error setting icon: %v", err))
			return
		}
	case "moewalls":
		msgID := sendMessageWithID(s, m, "🎬 Fetching random icon from MoeWalls...")
		result, e := mwc.GetRandomVideoWithStatus(func(status string) {
			editMessage(s, m, msgID, status)
		})
		if e != nil {
			editMessage(s, m, msgID, fmt.Sprintf("❌ Error: %v", e))
			return
		}
		editMessage(s, m, msgID, "⬆️ Uploading to Discord...")
		if err := setAnimatedIcon(s, m, result.GIFData, guildID); err != nil {
			editMessage(s, m, msgID, fmt.Sprintf("❌ Error setting icon: %v", err))
			return
		}
		editMessage(s, m, msgID, fmt.Sprintf("✅ Icon updated!\n🔗 **Wallpaper:** %s\n🔗 **Video:** %s", result.WallpaperURL, result.VideoURL))
	default: // konachan
		iconKC := utils.NewKonachanClient(cfg.Konachan.IconTags, cfg.Konachan.Rating, cfg.Konachan.MinScore)
		img, e := iconKC.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		if err := downloadAndSetIconFromURL(s, m, img.FileURL, guildID); err != nil {
			sendMessage(s, m, fmt.Sprintf("Error setting icon: %v", err))
			return
		}
	}

	sendMessage(s, m, "Icon updated!")
}

func handleSetBanner(s *discordgo.Session, m *discordgo.MessageCreate, kc *utils.KonachanClient, zc *utils.ZerochanClient, whc *utils.WallhavenClient, mwc *utils.MoeWallsClient, cfg *config.Config, guildID string) {
	sendMessage(s, m, fmt.Sprintf("Fetching random banner from %s...", cfg.Source))

	switch cfg.Source {
	case "zerochan":
		entry, e := zc.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL := entry.GetImageURL()
		if imgURL == "" {
			sendMessage(s, m, "Error: No valid image URL found from zerochan")
			return
		}
		if err := downloadAndSetBannerFromURL(s, m, imgURL, guildID); err != nil {
			sendMessage(s, m, fmt.Sprintf("Error setting banner: %v", err))
			return
		}
	case "wallhaven":
		entry, e := whc.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		imgURL := entry.GetImageURL()
		if imgURL == "" {
			sendMessage(s, m, "Error: No valid image URL found from wallhaven")
			return
		}
		if err := downloadAndSetBannerFromURL(s, m, imgURL, guildID); err != nil {
			sendMessage(s, m, fmt.Sprintf("Error setting banner: %v", err))
			return
		}
	case "moewalls":
		msgID := sendMessageWithID(s, m, "🎬 Fetching random banner from MoeWalls...")
		result, e := mwc.GetRandomVideoWithStatus(func(status string) {
			editMessage(s, m, msgID, status)
		})
		if e != nil {
			editMessage(s, m, msgID, fmt.Sprintf("❌ Error: %v", e))
			return
		}
		editMessage(s, m, msgID, "⬆️ Uploading to Discord...")
		if err := setAnimatedBanner(s, m, result.GIFData, guildID); err != nil {
			editMessage(s, m, msgID, fmt.Sprintf("❌ Error setting banner: %v", err))
			return
		}
		editMessage(s, m, msgID, fmt.Sprintf("✅ Banner updated!\n🔗 **Wallpaper:** %s\n🔗 **Video:** %s", result.WallpaperURL, result.VideoURL))
	default: // konachan
		bannerKC := utils.NewKonachanClient(cfg.Konachan.BannerTags, cfg.Konachan.Rating, cfg.Konachan.MinScore)
		img, e := bannerKC.GetRandomImage()
		if e != nil {
			sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", e))
			return
		}
		if err := downloadAndSetBannerFromURL(s, m, img.FileURL, guildID); err != nil {
			sendMessage(s, m, fmt.Sprintf("Error setting banner: %v", err))
			return
		}
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
	guildEditLimiter.Wait()
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIGuildEdit)
	}
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
	guildEditLimiter.Wait()
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIGuildEdit)
	}
	return err
}

func downloadAndSetBanner(s *discordgo.Session, m *discordgo.MessageCreate, img *utils.KonachanPost, guildID string) error {
	data, err := downloadImage(img.FileURL)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(data))
	guildEditLimiter.Wait()
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Banner: dataURI,
	})
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIGuildEdit)
	}
	return err
}

func downloadAndSetBannerFromURL(s *discordgo.Session, m *discordgo.MessageCreate, imgURL string, guildID string) error {
	data, err := downloadImage(imgURL)
	if err != nil {
		return err
	}

	// Compress if over 10MB limit
	compressed, err := utils.CompressToUnderLimit(data)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(compressed))
	guildEditLimiter.Wait()
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Banner: dataURI,
	})
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIGuildEdit)
	}
	return err
}

func setAnimatedIcon(s *discordgo.Session, m *discordgo.MessageCreate, gifData []byte, guildID string) error {
	dataURI := fmt.Sprintf("data:image/gif;base64,%s", encodeBase64(gifData))
	guildEditLimiter.Wait()
	_, err := s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIGuildEdit)
	}
	return err
}

func setAnimatedBanner(s *discordgo.Session, m *discordgo.MessageCreate, gifData []byte, guildID string) error {
	dataURI := fmt.Sprintf("data:image/gif;base64,%s", encodeBase64(gifData))
	guildEditLimiter.Wait()
	_, err := s.GuildEdit(guildID, &discordgo.GuildParams{
		Banner: dataURI,
	})
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIGuildEdit)
	}
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
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIGuild)
	}
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
		"- Interval: `%s`\n\n"+
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
		formatDuration(time.Duration(cfg.Auto.Interval)*time.Second),
		cfg.Konachan.Rating,
		cfg.Konachan.MinScore,
		m.GuildID)

	sendMessage(s, m, content)
}

func handleSetInterval(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	duration, err := parseDuration(value)
	if err != nil {
		sendMessage(s, m, fmt.Sprintf("Invalid format. Use: `1h30m`, `2d12h`, `30m`, `90s`\nMinimum: 1 minute"))
		return
	}

	seconds := int(duration.Seconds())
	if seconds < 60 {
		sendMessage(s, m, "Interval must be at least **1 minute** to avoid Discord API rate limits.")
		return
	}

	cfg.Auto.Interval = seconds
	config.SaveConfig(cfg)
	sendMessage(s, m, fmt.Sprintf("Interval set to **%s**", formatDuration(duration)))
}

// parseDuration parses human-readable duration like "1h30m", "2d12h", "30m", "90s"
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	var total time.Duration
	current := ""

	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			current += string(ch)
		} else if ch == 'y' || ch == 'Y' {
			if current == "" {
				return 0, fmt.Errorf("missing number before 'y'")
			}
			n, err := fmt.Sscanf(current, "%d", new(int))
			if n != 1 || err != nil {
				return 0, fmt.Errorf("invalid number")
			}
			var num int
			fmt.Sscanf(current, "%d", &num)
			total += time.Duration(num) * 365 * 24 * time.Hour
			current = ""
		} else if ch == 'M' {
			if current == "" {
				return 0, fmt.Errorf("missing number before 'M'")
			}
			var num int
			fmt.Sscanf(current, "%d", &num)
			total += time.Duration(num) * 30 * 24 * time.Hour
			current = ""
		} else if ch == 'd' || ch == 'D' {
			if current == "" {
				return 0, fmt.Errorf("missing number before 'd'")
			}
			var num int
			fmt.Sscanf(current, "%d", &num)
			total += time.Duration(num) * 24 * time.Hour
			current = ""
		} else if ch == 'h' || ch == 'H' {
			if current == "" {
				return 0, fmt.Errorf("missing number before 'h'")
			}
			var num int
			fmt.Sscanf(current, "%d", &num)
			total += time.Duration(num) * time.Hour
			current = ""
		} else if ch == 'm' {
			if current == "" {
				return 0, fmt.Errorf("missing number before 'm'")
			}
			var num int
			fmt.Sscanf(current, "%d", &num)
			total += time.Duration(num) * time.Minute
			current = ""
		} else if ch == 's' || ch == 'S' {
			if current == "" {
				return 0, fmt.Errorf("missing number before 's'")
			}
			var num int
			fmt.Sscanf(current, "%d", &num)
			total += time.Duration(num) * time.Second
			current = ""
		} else {
			return 0, fmt.Errorf("invalid character: %c", ch)
		}
	}

	// If no unit specified, assume seconds
	if current != "" {
		var num int
		fmt.Sscanf(current, "%d", &num)
		total += time.Duration(num) * time.Second
	}

	if total <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}

	return total, nil
}

// formatDuration formats duration to human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	parts := []string{}
	seconds := int(d.Seconds())

	if seconds >= 86400 {
		days := seconds / 86400
		parts = append(parts, fmt.Sprintf("%dd", days))
		seconds %= 86400
	}
	if seconds >= 3600 {
		hours := seconds / 3600
		parts = append(parts, fmt.Sprintf("%dh", hours))
		seconds %= 3600
	}
	if seconds >= 60 {
		minutes := seconds / 60
		parts = append(parts, fmt.Sprintf("%dm", minutes))
		seconds %= 60
	}
	if seconds > 0 && len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	return strings.Join(parts, "")
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
	case "konachan", "zerochan", "wallhaven", "moewalls":
		cfg.Source = value
		config.SaveConfig(cfg)
		sendMessage(s, m, fmt.Sprintf("Image source set to `%s`", value))
	default:
		sendMessage(s, m, "Available sources: `konachan`, `zerochan`, `wallhaven`, `moewalls`")
	}
}

func handleRPSCheck(s *discordgo.Session, m *discordgo.MessageCreate) {
	if rpsMonitor == nil {
		sendMessage(s, m, "RPS monitor not initialized")
		return
	}
	summary := rpsMonitor.GetSummary(30) // last 30 seconds
	sendMessage(s, m, fmt.Sprintf("```%s```", summary))
}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, content string) {
	s.ChannelMessageSend(m.ChannelID, content)
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIChannelMessageSend)
	}
}

func editMessage(s *discordgo.Session, m *discordgo.MessageCreate, msgID string, content string) {
	s.ChannelMessageEdit(m.ChannelID, msgID, content)
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIChannelMessageEdit)
	}
}

func sendMessageWithID(s *discordgo.Session, m *discordgo.MessageCreate, content string) string {
	msg, _ := s.ChannelMessageSend(m.ChannelID, content)
	if rpsMonitor != nil {
		rpsMonitor.Record(utils.APIChannelMessageSend)
	}
	if msg != nil {
		return msg.ID
	}
	return ""
}

func hasManageServerOrAdmin(s *discordgo.Session, userID, guildID string) bool {
	// Try local state cache first (no API call)
	member, err := s.State.Member(guildID, userID)
	if err == nil {
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIStateMember)
		}
	} else {
		// Cache miss: fetch from API (1 request)
		member, err = s.GuildMember(guildID, userID)
		if err != nil {
			return false
		}
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIGuildMember)
		}
	}

	// Check roles for Administrator or ManageGuild permissions
	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			continue
		}
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIStateRole)
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

func InteractionHandler(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, kc *utils.KonachanClient, zc *utils.ZerochanClient, whc *utils.WallhavenClient, guildID string) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// Only process interactions from the configured guild
	if i.GuildID != guildID {
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
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIInteractionRespond)
		}
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
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIInteractionRespond)
		}
	case "setbanner":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Fetching random banner...",
			},
		})
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIInteractionRespond)
		}
	case "boost":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Checking boost level...",
			},
		})
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIInteractionRespond)
		}
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
		if rpsMonitor != nil {
			rpsMonitor.Record(utils.APIInteractionRespond)
		}
	}
}

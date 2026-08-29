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

func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, wc *utils.WallhavenClient, guildID string) {
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

	switch cmd {
	case "help":
		sendHelp(s, m, cfg)
	case "seticon":
		handleSetIcon(s, m, wc, cfg, guildID)
	case "setbanner":
		handleSetBanner(s, m, wc, cfg, guildID)
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
		"`%sinterval <seconds>` - Set auto change interval\n\n"+
		"**Image Commands:**\n"+
		"`%sseticon` - Set random icon from wallhaven\n"+
		"`%ssetbanner` - Set random banner from wallhaven\n\n"+
		"**Toggle:**\n"+
		"`%stoggle icon` - Toggle auto icon\n"+
		"`%stoggle banner` - Toggle auto banner\n\n"+
		"Powered by wallhaven.cc",
		prefix, prefix, prefix, prefix, prefix, prefix, prefix)

	sendMessage(s, m, content)
}

func handleSetIcon(s *discordgo.Session, m *discordgo.MessageCreate, wc *utils.WallhavenClient, cfg *config.Config, guildID string) {
	sendMessage(s, m, "Fetching random icon from wallhaven...")

	iconWC := utils.NewWallhavenClient(
		wc.APIKey, wc.Categories, wc.Purity, wc.Sorting, cfg.Wallhaven.IconRatio,
	)

	img, err := iconWC.GetRandomImage()
	if err != nil {
		sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", err))
		return
	}

	if err := downloadAndSetIcon(s, m, img, guildID); err != nil {
		sendMessage(s, m, fmt.Sprintf("Error setting icon: %v", err))
		return
	}

	sendMessage(s, m, fmt.Sprintf("Icon updated! Source: %s", img.URL))
}

func handleSetBanner(s *discordgo.Session, m *discordgo.MessageCreate, wc *utils.WallhavenClient, cfg *config.Config, guildID string) {
	sendMessage(s, m, "Fetching random banner from wallhaven...")

	bannerWC := utils.NewWallhavenClient(
		wc.APIKey, wc.Categories, wc.Purity, wc.Sorting, cfg.Wallhaven.BannerRatio,
	)

	img, err := bannerWC.GetRandomImage()
	if err != nil {
		sendMessage(s, m, fmt.Sprintf("Error fetching image: %v", err))
		return
	}

	if err := downloadAndSetBanner(s, m, img, guildID); err != nil {
		sendMessage(s, m, fmt.Sprintf("Error setting banner: %v", err))
		return
	}

	sendMessage(s, m, fmt.Sprintf("Banner updated! Source: %s", img.URL))
}

func downloadAndSetIcon(s *discordgo.Session, m *discordgo.MessageCreate, img *utils.WallhavenImage, guildID string) error {
	data, err := downloadImage(img.Path)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/png;base64,%s", encodeBase64(data))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	return err
}

func downloadAndSetBanner(s *discordgo.Session, m *discordgo.MessageCreate, img *utils.WallhavenImage, guildID string) error {
	data, err := downloadImage(img.Path)
	if err != nil {
		return err
	}

	dataURI := fmt.Sprintf("data:image/png;base64,%s", encodeBase64(data))
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
	content := fmt.Sprintf("**Bot Configuration**\n\n"+
		"**Prefix:** %s\n"+
		"**Status:** %s\n"+
		"**Auto Icon:** %v\n"+
		"**Auto Banner:** %v\n"+
		"**Interval:** %d seconds\n"+
		"**Wallhaven Sorting:** %s\n"+
		"**Banner Ratio:** %s\n"+
		"**Icon Ratio:** %s",
		cfg.Bot.Prefix,
		cfg.Bot.Status,
		cfg.Auto.IconEnabled,
		cfg.Auto.BannerEnabled,
		cfg.Auto.Interval,
		cfg.Wallhaven.Sorting,
		cfg.Wallhaven.BannerRatio,
		cfg.Wallhaven.IconRatio)

	sendMessage(s, m, content)
}

func handleSetInterval(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, value string) {
	var interval int
	if _, err := fmt.Sscanf(value, "%d", &interval); err != nil || interval < 60 {
		sendMessage(s, m, "Interval must be a number >= 60 seconds")
		return
	}
	cfg.Auto.Interval = interval
	sendMessage(s, m, fmt.Sprintf("Interval set to %d seconds", interval))
}

func handleToggle(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, target string) {
	switch strings.ToLower(target) {
	case "icon":
		cfg.Auto.IconEnabled = !cfg.Auto.IconEnabled
		sendMessage(s, m, fmt.Sprintf("Auto icon: %v", cfg.Auto.IconEnabled))
	case "banner":
		cfg.Auto.BannerEnabled = !cfg.Auto.BannerEnabled
		sendMessage(s, m, fmt.Sprintf("Auto banner: %v", cfg.Auto.BannerEnabled))
	default:
		sendMessage(s, m, "Usage: !toggle <icon|banner>")
	}
}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, content string) {
	s.ChannelMessageSend(m.ChannelID, content)
}

func InteractionHandler(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, wc *utils.WallhavenClient, guildID string) {
	if i.Type != discordgo.InteractionApplicationCommand {
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

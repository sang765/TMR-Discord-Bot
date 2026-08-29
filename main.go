package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"TMR-Discord-Bot/config"
	"TMR-Discord-Bot/handlers"
	"TMR-Discord-Bot/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found")
	}

	env := config.LoadEnv()
	if env.DiscordToken == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}
	if env.GuildID == "" {
		log.Fatal("GUILD_ID is required")
	}

	cfg, err := config.LoadConfig("config/config.yml")
	if err != nil {
		slog.Warn("Failed to load config, using defaults", slog.Any("error", err))
		cfg = &config.Config{
			Bot: config.BotConfig{
				Prefix: "!",
				Status: "TMR Auto Server",
			},
			Auto: config.AutoConfig{
				IconEnabled:   true,
				BannerEnabled: true,
				Interval:      300,
			},
			Konachan: config.KonachanConfig{
				IconTags:   "1girl",
				BannerTags: "landscape",
				Rating:     "s",
				MinScore:   50,
			},
		}
	}

	konachanClient := utils.NewKonachanClient("", cfg.Konachan.Rating, cfg.Konachan.MinScore)

	dg, err := discordgo.New("Bot " + env.DiscordToken)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	dg.Identify.Intents = discordgo.IntentGuilds |
		discordgo.IntentGuildMessages |
		discordgo.IntentMessageContent

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		slog.Info("Bot is ready!", slog.String("username", r.User.Username))
	})

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		handlers.MessageHandler(s, m, cfg, konachanClient, env.GuildID)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handlers.InteractionHandler(s, i, cfg, konachanClient, env.GuildID)
	})

	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening connection: ", err)
	}

	slog.Info("Bot is starting...",
		slog.String("prefix", cfg.Bot.Prefix),
		slog.Bool("auto_icon", cfg.Auto.IconEnabled),
		slog.Bool("auto_banner", cfg.Auto.BannerEnabled),
		slog.Int("interval", cfg.Auto.Interval),
		slog.String("icon_tags", cfg.Konachan.IconTags),
		slog.String("banner_tags", cfg.Konachan.BannerTags),
	)

	if cfg.Auto.IconEnabled || cfg.Auto.BannerEnabled {
		go handlers.AutoChangeLoop(context.TODO(), dg, env.GuildID, cfg, konachanClient)
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	fmt.Println("\nShutting down bot...")
	dg.Close()
	time.Sleep(2 * time.Second)
}

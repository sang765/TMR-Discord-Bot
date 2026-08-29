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
			Wallhaven: config.WallhavenConfig{
				APIKey: "",
				Categories: config.WallhavenCategories{
					Anime:   1,
					General: 1,
					People:  0,
				},
				Purity: config.WallhavenPurity{
					SFW:     1,
					Sketchy: 0,
					NSFW:    0,
				},
				Sorting:     "random",
				BannerRatio: "16x9",
				IconRatio:   "1x1",
			},
		}
	}

	categories := utils.BuildCategoriesString(
		cfg.Wallhaven.Categories.Anime,
		cfg.Wallhaven.Categories.General,
		cfg.Wallhaven.Categories.People,
	)
	purity := utils.BuildPurityString(
		cfg.Wallhaven.Purity.SFW,
		cfg.Wallhaven.Purity.Sketchy,
		cfg.Wallhaven.Purity.NSFW,
	)

	wallhavenClient := utils.NewWallhavenClient(
		cfg.Wallhaven.APIKey,
		categories,
		purity,
		cfg.Wallhaven.Sorting,
		"",
	)

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
		handlers.MessageHandler(s, m, cfg, wallhavenClient, env.GuildID)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handlers.InteractionHandler(s, i, cfg, wallhavenClient, env.GuildID)
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
	)

	if cfg.Auto.IconEnabled || cfg.Auto.BannerEnabled {
		go handlers.AutoChangeLoop(context.TODO(), dg, env.GuildID, cfg, wallhavenClient)
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	fmt.Println("\nShutting down bot...")
	dg.Close()
	time.Sleep(2 * time.Second)
}

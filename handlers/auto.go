package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"TMR-Discord-Bot/config"
	"TMR-Discord-Bot/utils"

	"github.com/bwmarrin/discordgo"
)

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func AutoChangeLoop(ctx context.Context, s *discordgo.Session, guildID string, cfg *config.Config, wc *utils.WallhavenClient) {
	interval := time.Duration(cfg.Auto.Interval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("Auto change loop started",
		slog.Duration("interval", interval),
		slog.Bool("icon", cfg.Auto.IconEnabled),
		slog.Bool("banner", cfg.Auto.BannerEnabled),
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Auto change loop stopped")
			return
		case <-ticker.C:
			if cfg.Auto.IconEnabled {
				changeIcon(s, guildID, cfg, wc)
			}
			time.Sleep(5 * time.Second)
			if cfg.Auto.BannerEnabled {
				changeBanner(s, guildID, cfg, wc)
			}
		}
	}
}

func changeIcon(s *discordgo.Session, guildID string, cfg *config.Config, wc *utils.WallhavenClient) {
	slog.Info("Changing server icon...")

	iconWC := utils.NewWallhavenClient(
		wc.APIKey, wc.Categories, wc.Purity, wc.Sorting, cfg.Wallhaven.IconRatio,
	)

	img, err := iconWC.GetRandomImage()
	if err != nil {
		slog.Error("Failed to fetch icon image", slog.Any("error", err))
		return
	}

	data, err := downloadImageForAuto(img.Path)
	if err != nil {
		slog.Error("Failed to download icon image", slog.Any("error", err))
		return
	}

	dataURI := fmt.Sprintf("data:image/png;base64,%s", encodeBase64(data))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	if err != nil {
		slog.Error("Failed to set icon", slog.Any("error", err))
		return
	}

	slog.Info("Server icon updated", slog.String("source", img.URL))
}

func changeBanner(s *discordgo.Session, guildID string, cfg *config.Config, wc *utils.WallhavenClient) {
	slog.Info("Changing server banner...")

	bannerWC := utils.NewWallhavenClient(
		wc.APIKey, wc.Categories, wc.Purity, wc.Sorting, cfg.Wallhaven.BannerRatio,
	)

	img, err := bannerWC.GetRandomImage()
	if err != nil {
		slog.Error("Failed to fetch banner image", slog.Any("error", err))
		return
	}

	data, err := downloadImageForAuto(img.Path)
	if err != nil {
		slog.Error("Failed to download banner image", slog.Any("error", err))
		return
	}

	dataURI := fmt.Sprintf("data:image/png;base64,%s", encodeBase64(data))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Banner: dataURI,
	})
	if err != nil {
		slog.Error("Failed to set banner", slog.Any("error", err))
		return
	}

	slog.Info("Server banner updated", slog.String("source", img.URL))
}

func downloadImageForAuto(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

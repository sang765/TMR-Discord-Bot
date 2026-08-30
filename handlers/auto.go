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

func AutoChangeLoop(ctx context.Context, s *discordgo.Session, guildID string, cfg *config.Config, kc *utils.KonachanClient, zc *utils.ZerochanClient) {
	interval := time.Duration(cfg.Auto.Interval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("Auto change loop started",
		slog.Duration("interval", interval),
		slog.Bool("banner", cfg.Auto.BannerEnabled),
		slog.String("source", cfg.Source),
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Auto change loop stopped")
			return
		case <-ticker.C:
			if cfg.Auto.BannerEnabled {
				changeBanner(s, guildID, cfg, kc, zc)
			}
		}
	}
}

func changeIcon(s *discordgo.Session, guildID string, cfg *config.Config, kc *utils.KonachanClient, zc *utils.ZerochanClient) {
	slog.Info("Changing server icon...")

	var imgURL string

	switch cfg.Source {
	case "zerochan":
		entry, err := zc.GetRandomImage()
		if err != nil {
			slog.Error("Failed to fetch icon from zerochan", slog.Any("error", err))
			return
		}
		imgURL = entry.GetImageURL()
	default: // konachan
		iconKC := utils.NewKonachanClient(cfg.Konachan.IconTags, cfg.Konachan.Rating, cfg.Konachan.MinScore)
		img, err := iconKC.GetRandomImage()
		if err != nil {
			slog.Error("Failed to fetch icon from konachan", slog.Any("error", err))
			return
		}
		imgURL = img.FileURL
	}

	data, err := downloadImageForAuto(imgURL)
	if err != nil {
		slog.Error("Failed to download icon image", slog.Any("error", err))
		return
	}

	cropped, err := utils.CropBytesToSquare(data, 512)
	if err != nil {
		slog.Error("Failed to crop icon image", slog.Any("error", err))
		return
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(cropped))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Icon: dataURI,
	})
	if err != nil {
		slog.Error("Failed to set icon", slog.Any("error", err))
		return
	}

	slog.Info("Server icon updated")
}

func changeBanner(s *discordgo.Session, guildID string, cfg *config.Config, kc *utils.KonachanClient, zc *utils.ZerochanClient) {
	slog.Info("Changing server banner...")

	var imgURL string

	switch cfg.Source {
	case "zerochan":
		entry, err := zc.GetRandomImage()
		if err != nil {
			slog.Error("Failed to fetch banner from zerochan", slog.Any("error", err))
			return
		}
		imgURL = entry.GetImageURL()
	default: // konachan
		bannerKC := utils.NewKonachanClient(cfg.Konachan.BannerTags, cfg.Konachan.Rating, cfg.Konachan.MinScore)
		img, err := bannerKC.GetRandomImage()
		if err != nil {
			slog.Error("Failed to fetch banner from konachan", slog.Any("error", err))
			return
		}
		imgURL = img.FileURL
	}

	data, err := downloadImageForAuto(imgURL)
	if err != nil {
		slog.Error("Failed to download banner image", slog.Any("error", err))
		return
	}

	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", encodeBase64(data))
	_, err = s.GuildEdit(guildID, &discordgo.GuildParams{
		Banner: dataURI,
	})
	if err != nil {
		slog.Error("Failed to set banner", slog.Any("error", err))
		return
	}

	slog.Info("Server banner updated")
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

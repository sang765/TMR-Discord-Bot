package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type BotConfig struct {
	Prefix string `yaml:"prefix"`
	Status string `yaml:"status"`
}

type AutoConfig struct {
	IconEnabled  bool  `yaml:"icon_enabled"`
	BannerEnabled bool `yaml:"banner_enabled"`
	Interval     int   `yaml:"interval"`
}

type WallhavenCategories struct {
	Anime   int `yaml:"anime"`
	General int `yaml:"general"`
	People  int `yaml:"people"`
}

type WallhavenPurity struct {
	SFW      int `yaml:"sfw"`
	Sketchy  int `yaml:"sketchy"`
	NSFW     int `yaml:"nsfw"`
}

type WallhavenConfig struct {
	APIKey      string              `yaml:"api_key"`
	Categories  WallhavenCategories `yaml:"categories"`
	Purity      WallhavenPurity     `yaml:"purity"`
	Sorting     string              `yaml:"sorting"`
	BannerRatio string              `yaml:"banner_ratio"`
	IconRatio   string              `yaml:"icon_ratio"`
}

type Config struct {
	Bot       BotConfig       `yaml:"bot"`
	Auto      AutoConfig      `yaml:"auto"`
	Wallhaven WallhavenConfig `yaml:"wallhaven"`
}

type EnvConfig struct {
	DiscordToken string
	GuildID      string
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Bot: BotConfig{
			Prefix: "!",
			Status: "TMR Auto Server",
		},
		Auto: AutoConfig{
			IconEnabled:   true,
			BannerEnabled: true,
			Interval:      300,
		},
		Wallhaven: WallhavenConfig{
			APIKey: "",
			Categories: WallhavenCategories{
				Anime:   1,
				General: 1,
				People:  0,
			},
			Purity: WallhavenPurity{
				SFW:     1,
				Sketchy: 0,
				NSFW:    0,
			},
			Sorting:     "random",
			BannerRatio: "16x9",
			IconRatio:   "1x1",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func LoadEnv() *EnvConfig {
	return &EnvConfig{
		DiscordToken: os.Getenv("DISCORD_TOKEN"),
		GuildID:      os.Getenv("GUILD_ID"),
	}
}

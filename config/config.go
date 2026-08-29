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
	IconEnabled   bool `yaml:"icon_enabled"`
	BannerEnabled bool `yaml:"banner_enabled"`
	Interval      int  `yaml:"interval"`
}

type KonachanConfig struct {
	IconTags  string `yaml:"icon_tags"`
	BannerTags string `yaml:"banner_tags"`
	Rating    string `yaml:"rating"`
	MinScore  int    `yaml:"min_score"`
}

type Config struct {
	Bot      BotConfig      `yaml:"bot"`
	Auto     AutoConfig     `yaml:"auto"`
	Konachan KonachanConfig `yaml:"konachan"`
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
		Konachan: KonachanConfig{
			IconTags:   "1girl",
			BannerTags: "landscape",
			Rating:     "s",
			MinScore:   50,
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

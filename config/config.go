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
	IconTags   string `yaml:"icon_tags"`
	BannerTags string `yaml:"banner_tags"`
	Rating     string `yaml:"rating"`
	MinScore   int    `yaml:"min_score"`
}

type ZerochanConfig struct {
	Tags string `yaml:"tags"`
}

type MoeWallsConfig struct {
	Enabled bool `yaml:"enabled"`
}

type WallhavenConfig struct {
	Tags string `yaml:"tags"`
}

type Config struct {
	Bot       BotConfig       `yaml:"bot"`
	Auto      AutoConfig      `yaml:"auto"`
	Source    string          `yaml:"source"` // konachan, zerochan, wallhaven
	Konachan  KonachanConfig  `yaml:"konachan"`
	Zerochan  ZerochanConfig  `yaml:"zerochan"`
	Wallhaven WallhavenConfig `yaml:"wallhaven"`
	MoeWalls  MoeWallsConfig  `yaml:"moewalls"`

	// Internal: path to save config
	configPath string `yaml:"-"`
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
		Source: "konachan",
		Konachan: KonachanConfig{
			IconTags:   "1girl",
			BannerTags: "landscape",
			Rating:     "s",
			MinScore:   50,
		},
		Zerochan: ZerochanConfig{
			Tags: "1girl",
		},
		MoeWalls: MoeWallsConfig{
			Enabled: false,
		},
		configPath: path,
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveConfig writes the current config to the YAML file
func SaveConfig(cfg *Config) error {
	if cfg.configPath == "" {
		return nil
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(cfg.configPath, data, 0644)
}

func LoadEnv() *EnvConfig {
	return &EnvConfig{
		DiscordToken: os.Getenv("DISCORD_TOKEN"),
		GuildID:      os.Getenv("GUILD_ID"),
	}
}

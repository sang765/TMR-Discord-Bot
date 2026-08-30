package utils

import (
	"github.com/bwmarrin/discordgo"
)

// GuildBoostInfo contains information about server boost status
type GuildBoostInfo struct {
	Level           int
	BoostCount      int
	HasBanner       bool
	HasAnimatedBanner bool
	HasAnimatedIcon bool
}

// GetGuildBoostInfo returns boost information for a guild
func GetGuildBoostInfo(s *discordgo.Session, guildID string) (*GuildBoostInfo, error) {
	guild, err := s.Guild(guildID)
	if err != nil {
		return nil, err
	}

	info := &GuildBoostInfo{
		Level:      int(guild.PremiumTier),
		BoostCount: guild.PremiumSubscriptionCount,
	}

	// Check guild features
	for _, feature := range guild.Features {
		switch feature {
		case "BANNER":
			info.HasBanner = true
		case "ANIMATED_BANNER":
			info.HasAnimatedBanner = true
		case "ANIMATED_ICON":
			info.HasAnimatedIcon = true
		}
	}

	return info, nil
}

// CanSetBanner returns true if guild can have a banner (Tier 2+)
func (info *GuildBoostInfo) CanSetBanner() bool {
	return info.Level >= 2 || info.HasBanner
}

// CanSetAnimatedBanner returns true if guild can have animated banner (Tier 3+)
func (info *GuildBoostInfo) CanSetAnimatedBanner() bool {
	return info.Level >= 3 || info.HasAnimatedBanner
}

// CanSetAnimatedIcon returns true if guild can have animated icon
func (info *GuildBoostInfo) CanSetAnimatedIcon() bool {
	return info.HasAnimatedIcon
}

// GetBoostEmoji returns an emoji representing the boost level
func (info *GuildBoostInfo) GetBoostEmoji() string {
	switch info.Level {
	case 0:
		return "⚪"
	case 1:
		return "Bronze"
	case 2:
		return "Silver"
	case 3:
		return "Gold"
	default:
		return "Unknown"
	}
}

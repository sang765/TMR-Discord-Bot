package main

// Chat streak: cấp role cho người chat đủ N ngày lịch VN liên tiếp.
// Chạy định kỳ bằng REST — không cần gateway intent, bot vẫn giữ REST-only.

import (
	"log"
	"strconv"
	"time"

	"tmr-bot/discord"
	"tmr-bot/store"
)

func itoa(n int) string { return strconv.Itoa(n) }

const streakStateFile = "data/streaks.json"

// runStreakCheck thực hiện 1 lần quét + gán role.
// dry=true: chỉ log, không gán role, không gửi thông báo.
func (a *app) runStreakCheck(dry bool) {
	if !a.cfg.Streak.Enabled || a.cfg.Streak.RoleID == "" {
		return
	}
	need := a.cfg.Streak.Days
	if need <= 0 {
		need = 3
	}
	backfill := a.cfg.Streak.BackfillDays
	if backfill <= 0 {
		backfill = 7
	}
	start := time.Now()
	cutoff := time.Now().Add(-time.Duration(backfill) * 24 * time.Hour)
	// trước = snowflake của "ngay bây giờ" — lấy mới nhất trước, rồi lùi dần bằng before
	pageBefore := store.SnowflakeBefore(time.Now().Add(time.Minute))
	today := store.VNDay(time.Now())

	st, err := store.OpenStreak(streakStateFile)
	if err != nil {
		log.Printf("streak: open store: %v", err)
		return
	}

	excluded := map[string]bool{}
	for _, id := range a.cfg.Streak.Exclude {
		excluded[id] = true
	}
	channels, err := a.dc.ListGuildChannels(a.cfg.GuildID)
	if err != nil {
		log.Printf("streak: list channels: %v", err)
		return
	}

	msgCount := 0
	for _, ch := range channels {
		if ch.NSFW || excluded[ch.ID] {
			continue
		}
		oldestInPage := ""
		for page := 0; page < 50; page++ { // hard cap ~5000 msg/kênh/lần
			msgs, err := a.dc.ListMessages(ch.ID, pageBefore, "", 100)
			if err != nil {
				log.Printf("streak: đọc #%s: %v", ch.Name, err)
				break
			}
			if len(msgs) == 0 {
				break
			}
			oldestInPage = msgs[len(msgs)-1].ID
			for _, m := range msgs {
				if m.Author.Bot || m.Author.System || m.Author.ID == "" {
					continue
				}
				ts, err := time.Parse(time.RFC3339, m.Timestamp)
				if err != nil || ts.Before(cutoff) {
					continue
				}
				st.MarkDay(m.Author.ID, store.VNDay(ts))
				msgCount++
			}
			// nếu tin cuối cùng trong trang đã cũ hơn cutoff thì dừng kênh này
			lastTs, ok := store.ParseSnowflakeTime(oldestInPage)
			if !ok || lastTs.Before(cutoff) {
				break
			}
			pageBefore = oldestInPage
		}
		pageBefore = store.SnowflakeBefore(time.Now().Add(time.Minute)) // reset cho kênh sau
	}

	granted := 0
	for _, userID := range st.Qualified(need, today) {
		if st.IsAwarded(userID) {
			continue
		}
		if dry {
			log.Printf("streak: [dry] cấp role %s cho user %s (chuỗi %d ngày)", a.cfg.Streak.RoleID, userID, need)
			continue
		}
		if err := a.dc.AddRole(a.cfg.GuildID, userID, a.cfg.Streak.RoleID); err != nil {
			log.Printf("streak: gán role cho %s thất bại: %v", userID, err)
			continue
		}
		st.MarkAwarded(userID)
		granted++

		if a.cfg.Streak.Announce {
			ch := a.cfg.Streak.AnnounceCh
			if ch == "" {
				ch = a.cfg.Channels.Status
			}
			if ch != "" {
				msg := "🎉 <@" + userID + "> đã chat chuỗi " + itoa(need) + " ngày liên tục — nhận role **Con Dân Tích Cực**!"
				if _, err := a.dc.SendMessage(ch, discord.Message{Content: msg}); err != nil {
					log.Printf("streak: báo cấp role thất bại: %v", err)
				}
			}
		}
		time.Sleep(300 * time.Millisecond) // nhẹ nhàng với rate limit
	}

	if err := st.Save(streakStateFile); err != nil {
		log.Printf("streak: lưu state: %v", err)
	}
	log.Printf("streak: quét %d kênh, %d tin nhắn, cấp %d role trong %s", len(channels), msgCount, granted, time.Since(start).Round(time.Second))
}

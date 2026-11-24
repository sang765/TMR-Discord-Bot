package main

// Realtime streak: nối Discord Gateway, nhận MESSAGE_CREATE (không cần Message
// Content Intent vì chỉ dùng author.id + timestamp), mark ngày VN và cấp role
// ngay khi đủ N ngày liên tiếp. REST backfill vẫn chạy lúc khởi động trong
// streak.go để lo lịch sử cũ.

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"tmr-bot/discord"
	"tmr-bot/store"
)

// RealtimeMessage là subset của MESSAGE_CREATE mà streak cần.
type RealtimeMessage struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
	Member json.RawMessage `json:"member"`
}

// gatewayIntents = 0 (không xin privileged intent). Discord vẫn gửi MESSAGE_CREATE
// kèm author.id + timestamp — đủ cho streak. Nếu sau này cần content thì bật
// Message Content Intent trong Developer Portal rồi đổi thành (1 << 15).
const gatewayIntents = 0

func (a *app) runGateway() {
	token := a.cfg.DiscordToken
	if token == "" || !a.cfg.Streak.Enabled {
		return
	}
	exclude := map[string]bool{}
	for _, id := range a.cfg.Streak.Exclude {
		exclude[id] = true
	}
	need := a.cfg.Streak.Days
	if need <= 0 {
		need = 3
	}
	st, err := store.OpenStreak(streakStateFile)
	if err != nil {
		log.Printf("gateway: open streak store: %v", err)
		return
	}
	// lấy danh sách kênh text hợp lệ (có thể lỗi → fallback chấp nhận mọi channel)
	valid := map[string]bool{}
	if chans, err := a.dc.ListGuildChannels(a.cfg.GuildID); err == nil {
		for _, ch := range chans {
			if !ch.NSFW && !exclude[ch.ID] {
				valid[ch.ID] = true
			}
		}
	}

	reconnectDelay := 2 * time.Second
	sess := &gatewaySessionState{}
	for {
		started := time.Now()
		err := a.gatewaySession(token, need, st, valid, exclude, sess)
		// Session sống đủ lâu (>= 1 phút) thì coi như ổn — reset backoff,
		// không để nó kẹt ở 1m4s như trước.
		if time.Since(started) >= time.Minute {
			reconnectDelay = 2 * time.Second
		}
		if err != nil {
			log.Printf("gateway: %v — reconnect sau %s (resume=%t)", err, reconnectDelay, sess.id != "")
			time.Sleep(reconnectDelay)
			if reconnectDelay < 30*time.Second {
				reconnectDelay *= 2
			}
		}
	}
}

// gatewaySessionState giữ session_id + seq giữa các lần reconnect để RESUME —
// nếu không, mỗi lần rớt mạng Android là mất toàn bộ sự kiện trong lúc offline.
type gatewaySessionState struct {
	id  string
	seq int
}

func (a *app) gatewaySession(token string, need int, st *store.StreakStore, valid, exclude map[string]bool, sess *gatewaySessionState) error {
	gw, err := a.dc.GetGatewayURL()
	if err != nil {
		return err
	}
	ws, err := discord.DialWS(gw+"?v=10&encoding=json", 15*time.Second)
	if err != nil {
		return err
	}
	defer ws.Close()
	if sess.id != "" && sess.seq > 0 {
		log.Printf("gateway: RESUME session=%s seq=%d", sess.id, sess.seq)
		if err := ws.SendText(discord.ResumePayload(token, sess.id, sess.seq)); err != nil {
			return err
		}
	} else if err := ws.SendText(discord.IdentifyPayload(token, gatewayIntents)); err != nil {
		return err
	}

	// Mặc định 41s; sẽ đổi theo heartbeat_interval thật từ HELLO.
	heartbeatInterval := 41 * time.Second
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	giveUp := time.NewTimer(8 * time.Second) // nếu không Hello trong 8s thì reconnect
	defer giveUp.Stop()

	for {
		ws.SetDeadline(time.Now().Add(35 * time.Second))
		select {
		case <-heartbeat.C:
			payload, _ := json.Marshal(map[string]any{"op": 1, "d": sess.seq})
			if err := ws.SendText(payload); err != nil {
				return err
			}
		case <-giveUp.C:
			return fmt.Errorf("không nhận HELLO từ gateway")
		default:
			b, err := ws.ReadText()
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue // deadline chỉ để chạy vòng heartbeat, không phải lỗi
				}
				return err
			}
			var ev discord.GatewayEvent
			if err := json.Unmarshal(b, &ev); err != nil {
				continue
			}
			if ev.S > 0 {
				sess.seq = ev.S
			}
			switch ev.Op {
			case 1: // gateway yêu cầu heartbeat ngay
				payload, _ := json.Marshal(map[string]any{"op": 1, "d": sess.seq})
				if err := ws.SendText(payload); err != nil {
					return err
				}
			case 9: // Invalid Session — session hết hạn, reset rồi IDENTIFY lại
				sess.id, sess.seq = "", 0
				log.Printf("gateway: invalid session — identify lại")
				if err := ws.SendText(discord.IdentifyPayload(token, gatewayIntents)); err != nil {
					return err
				}
			case 10: // Hello — đọc heartbeat_interval rồi reset ticker
				giveUp.Stop()
				var hello struct {
					HeartbeatInterval float64 `json:"heartbeat_interval"`
				}
				if json.Unmarshal(ev.D, &hello) == nil && hello.HeartbeatInterval > 0 {
					iv := time.Duration(hello.HeartbeatInterval) * time.Millisecond
					heartbeat.Reset(iv)
					heartbeatInterval = iv
					log.Printf("gateway: HELLO heartbeat=%s", iv)
				}
				// Gửi heartbeat ngay sau Hello (Discord yêu cầu)
				payload, _ := json.Marshal(map[string]any{"op": 1, "d": sess.seq})
				_ = ws.SendText(payload)
			case 0:
				switch ev.T {
				case "RESUMED":
					log.Printf("gateway: RESUMED thành công session=%s seq=%d", sess.id, sess.seq)
				case "READY":
					var d struct {
						SessionID string `json:"session_id"`
					}
					_ = json.Unmarshal(ev.D, &d)
					sess.id = d.SessionID
					log.Printf("gateway: READY session=%s intents=%d", sess.id, gatewayIntents)
				case "MESSAGE_CREATE":
					var m RealtimeMessage
					if err := json.Unmarshal(ev.D, &m); err != nil {
						continue
					}
					if m.Author.Bot || m.Author.ID == "" || m.GuildID != a.cfg.GuildID {
						continue
					}
					if len(valid) > 0 && !valid[m.ChannelID] {
						continue
					}
					if exclude[m.ChannelID] {
						continue
					}
					ts, err := time.Parse(time.RFC3339, m.Timestamp)
					if err != nil {
						continue
					}
					day := store.VNDay(ts)
					log.Printf("gateway: MSG user=%s channel=%s day=%s", m.Author.ID, m.ChannelID, day)
					st.MarkDay(m.Author.ID, day)
					if !st.IsAwarded(m.Author.ID) && st.Qualified(need, store.VNDay(time.Now())) != nil {
						// Qualified trả cả user khác — check lại chính user này
						if hasStreak(st, m.Author.ID, need) {
							a.grantRole(st, m.Author.ID, m.Author.Username)
						}
					}
					_ = st.Save(streakStateFile)
				}
			}
		}
	}
}

func hasStreak(st *store.StreakStore, userID string, need int) bool {
	for _, u := range st.Qualified(need, store.VNDay(time.Now())) {
		if u == userID {
			return true
		}
	}
	return false
}

func (a *app) grantRole(st *store.StreakStore, userID, username string) {
	if username == "" {
		username = a.dc.GetUsername(userID)
	}
	reason := fmt.Sprintf("Người dùng %s đã đạt chuỗi %d ngày liên tục. Đã cấp quyền Attach File và Embed Link.", username, a.cfg.Streak.Days)
	if a.dry {
		log.Printf("gateway: [dry] cấp role cho %s — reason: %s", userID, reason)
		return
	}
	if err := a.dc.AddRole(a.cfg.GuildID, userID, a.cfg.Streak.RoleID, reason); err != nil {
		log.Printf("gateway: gán role cho %s: %v", userID, err)
		return
	}
	st.MarkAwarded(userID)
	log.Printf("gateway: đã cấp role Con Dân Tích Cực cho user %s", userID)
	if a.cfg.Streak.Announce {
		ch := a.cfg.Streak.AnnounceCh
		if ch == "" {
			ch = a.cfg.Channels.Status
		}
		if ch != "" {
			msg := "🎉 <@" + userID + "> đã chat chuỗi " + itoa(a.cfg.Streak.Days) + " ngày liên tục — nhận role **Con Dân Tích Cực**!"
			if _, err := a.dc.SendMessage(ch, discord.Message{Content: msg}); err != nil {
				log.Printf("gateway: báo role: %v", err)
			}
		}
	}
}

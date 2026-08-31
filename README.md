# TMR Discord Bot

Discord bot auto thay đổi server banner từ [konachan.net](https://konachan.net) và [zerochan.net](https://zerochan.net)

## Tính năng

- Auto thay đổi server banner (Boost Level 2+)
- Random ảnh anime từ konachan.net hoặc zerochan.net
- Hỗ trợ MoeWalls (video → GIF cho animated icon/banner)
- Prefix commands (`!`)
- Slash commands
- Config file YAML
- Lọc theo tags, rating, score
- Permission check: Chỉ Manage Server hoặc Administrator mới dùng được

## Commands

| Command | Mô tả |
|---------|-------|
| `!help` | Hiển thị danh sách commands |
| `!seticon` | Set icon ngẫu nhiên từ source hiện tại |
| `!setbanner` | Set banner ngẫu nhiên từ source hiện tại |
| `!boost` | Kiểm tra boost level server |
| `!config` | Xem cấu hình bot |
| `!interval <seconds>` | Đặt interval auto change |
| `!source <konachan\|zerochan>` | Chọn image source |
| `!toggle icon/banner` | Bật/tắt auto change |
| `!toggleautoicon` | Toggle auto icon (alias) |
| `!toggleautobanner` | Toggle auto banner (alias) |

## Setup

### 1. Clone repo

```bash
git clone https://github.com/your-username/TMR-Discord-Bot.git
cd TMR-Discord-Bot
```

### 2. Tạo file `.env`

```env
DISCORD_TOKEN=your_bot_token_here
GUILD_ID=your_server_id_here
```

### 3. Config (tùy chọn)

Chỉnh sửa `config/config.yml`:

```yaml
bot:
  prefix: "!"
  status: "TMR Auto Server"

# Image source: konachan, zerochan
source: "konachan"

auto:
  banner_enabled: true
  interval: 300  # giây

konachan:
  icon_tags: "1girl"
  banner_tags: "landscape"
  rating: "s"  # s=safe, q=questionable, e=explicit
  min_score: 50

zerochan:
  tags: "1girl"

moewalls:
  enabled: false  # Cần ffmpeg cho video → GIF
```

### 4. Chạy bot

**Local:**
```bash
go build -o tmr-bot .
./tmr-bot
```

**Docker:**
```bash
docker build -t tmr-bot .
docker run -d --name tmr-bot --env-file .env tmr-bot
```

**Pterodactyl:**
- Upload tất cả file lên server
- Startup Command: `bash run.sh`
- Bot sẽ tự compile trên server

## Invite Bot

```
https://discord.com/oauth2/authorize?client_id=YOUR_APP_ID&scope=bot&permissions=8
```

## Yêu cầu

- Go 1.23+
- Discord Bot Token ([Discord Developer Portal](https://discord.com/developers/applications))
- Server Boost Level 2+ (cho banner)
- ffmpeg (nếu dùng MoeWalls cho animated icon/banner)

## License

MIT

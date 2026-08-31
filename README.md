# TMR Discord Bot

Discord bot auto thay đổi server banner và icon từ các nguồn ảnh anime.

## Nguồn ảnh

| Nguồn | Loại | Ghi chú |
|-------|------|---------|
| [konachan.net](https://konachan.net) | Ảnh tĩnh | Hỗ trợ tags, rating, score |
| [zerochan.net](https://zerochan.net) | Ảnh tĩnh | Hỗ trợ tags |
| [wallhaven.cc](https://wallhaven.cc) | Ảnh tĩnh | Hỗ trợ tags, purity, sorting |
| [moewalls.com](https://moewalls.com) | Video → GIF | Animated banner/icon, 1920x1080, ≤10MB |

## Tính năng

- Auto thay đổi server banner và icon (Boost Level 2+)
- Random ảnh/video từ 4 nguồn
- Hỗ trợ animated banner/icon (video → GIF via ffmpeg)
- Interval tuỳ chỉnh (hỗ trợ: `30m`, `1h30m`, `2d12h`)
- Live status updates khi đang xử lý
- Permission check: Chỉ Manage Server hoặc Administrator
- Config file YAML tự lưu khi thay đổi

## Commands

| Command | Mô tả |
|---------|-------|
| `!help` | Hiển thị danh sách commands |
| `!setbanner` | Set banner ngẫu nhiên từ source hiện tại |
| `!seticon` | Set icon ngẫu nhiên từ source hiện tại |
| `!boost` | Kiểm tra boost level server |
| `!config` | Xem cấu hình bot |
| `!interval <value>` | Đặt interval auto change (vd: `30m`, `1h30m`, `2d`) |
| `!source <name>` | Chọn image source: `konachan`, `zerochan`, `wallhaven`, `moewalls` |
| `!toggle icon\|banner` | Bật/tắt auto change |
| `!toggleautoicon` | Toggle auto icon (alias) |
| `!toggleautobanner` | Toggle auto banner (alias) |
| `!setprefix <char>` | Đặt prefix mới |
| `!setstatus <text>` | Đặt bot status |
| `!setrating <s\|q\|e>` | Đặt rating filter (konachan) |
| `!setscore <number>` | Đặt min score filter (konachan) |

## Setup

### 1. Clone repo

```bash
git clone https://github.com/sang765/TMR-Discord-Bot.git
cd TMR-Discord-Bot
```

### 2. Tạo file `.env`

```env
DISCORD_TOKEN=your_bot_token_here
GUILD_ID=your_server_id_here
GITHUB_TOKEN=ghp_xxxxxxxxxxxx  # Cần cho private repo (Pterodactyl)
```

### 3. Config

Chỉnh sửa `config/config.yml`:

```yaml
bot:
  prefix: "!"
  status: "TMR Auto Server"

# Nguồn ảnh: konachan, zerochan, wallhaven, moewalls
source: "konachan"

auto:
  banner_enabled: true
  icon_enabled: false
  interval: 1800  # giây (30 phút)

konachan:
  icon_tags: "1girl"
  banner_tags: "landscape"
  rating: "s"  # s=safe, q=questionable, e=explicit
  min_score: 50

zerochan:
  tags: "1girl"

wallhaven:
  tags: "landscape"
  purity: "100"  # 100=SFW, 010=Sketchy, 001=NSFW
  sorting: "random"

moewalls:
  enabled: true  # Cần ffmpeg
```

### 4. Cài ffmpeg (cho MoeWalls)

```bash
# Debian/Ubuntu
sudo apt install ffmpeg

# macOS
brew install ffmpeg

# Arch
sudo pacman -S ffmpeg
```

### 5. Chạy bot

**Local:**
```bash
go build -o tmr-bot .
./tmr-bot
```

**Pterodactyl:**
- Upload tất cả file lên server
- Thêm `GITHUB_TOKEN` vào `.env` (nếu repo private)
- Startup Command: `bash run.sh`
- Bot sẽ tự download Go, ffmpeg, compile và chạy

## Invite Bot

```
https://discord.com/oauth2/authorize?client_id=YOUR_APP_ID&scope=bot&permissions=8
```

## Yêu cầu

- Go 1.25+
- Discord Bot Token ([Discord Developer Portal](https://discord.com/developers/applications))
- Server Boost Level 2+ (cho animated banner)
- ffmpeg (cho MoeWalls video → GIF)
- GITHUB_TOKEN (cho Pterodactyl với private repo)

## License

MIT

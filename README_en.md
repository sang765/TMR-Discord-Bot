# TMR Discord Bot

Discord bot that auto-changes server banner from [konachan.net](https://konachan.net) and [zerochan.net](https://zerochan.net)

## Features

- Auto-change server banner (Boost Level 2+)
- Random anime images from konachan.net or zerochan.net
- MoeWalls support (video → GIF for animated icon/banner)
- Prefix commands (`!`)
- Slash commands
- YAML config file
- Filter by tags, rating, score
- Permission check: Only Manage Server or Administrator can use

## Commands

| Command | Description |
|---------|-------------|
| `!help` | Show list of commands |
| `!seticon` | Set random icon from current source |
| `!setbanner` | Set random banner from current source |
| `!boost` | Check server boost level |
| `!config` | View bot config |
| `!interval <seconds>` | Set auto change interval |
| `!source <konachan\|zerochan>` | Choose image source |
| `!toggle icon/banner` | Toggle auto change |
| `!toggleautoicon` | Toggle auto icon (alias) |
| `!toggleautobanner` | Toggle auto banner (alias) |

## Setup

### 1. Clone repo

```bash
git clone https://github.com/your-username/TMR-Discord-Bot.git
cd TMR-Discord-Bot
```

### 2. Create `.env` file

```env
DISCORD_TOKEN=your_bot_token_here
GUILD_ID=your_server_id_here
```

### 3. Config (optional)

Edit `config/config.yml`:

```yaml
bot:
  prefix: "!"
  status: "TMR Auto Server"

# Image source: konachan, zerochan, wallhaven, moewalls
source: "konachan"

auto:
  banner_enabled: true
  interval: 300  # seconds

konachan:
  icon_tags: "1girl"
  banner_tags: "landscape"
  rating: "s"  # s=safe, q=questionable, e=explicit
  min_score: 50

zerochan:
  tags: "1girl"

moewalls:
  enabled: false  # Requires ffmpeg for video → GIF
```

### 4. Run bot

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
- Upload all files to server
- Startup Command: `bash run.sh`
- Bot will auto-compile on server

## Invite Bot

```
https://discord.com/oauth2/authorize?client_id=YOUR_APP_ID&scope=bot&permissions=8
```

## Requirements

- Go 1.23+
- Discord Bot Token ([Discord Developer Portal](https://discord.com/developers/applications))
- Server Boost Level 2+ (for banner)
- ffmpeg (if using MoeWalls for animated icon/banner)

## License

MIT

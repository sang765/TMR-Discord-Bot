#!/bin/bash
# TMR Discord Bot — start script (container / VPS)
set -u

cd /home/container || exit 1

REPO_URL="https://github.com/sang765/TMR-Discord-Bot.git"

# ---------------------------------------------------------------
# 1) Lấy code mới nhất
# ---------------------------------------------------------------
if [ ! -d ".git" ]; then
    echo "Cloning from git..."
    git clone "$REPO_URL" /tmp/repo || { echo "git clone failed!"; exit 1; }
    cp -a /tmp/repo/. /home/container/
    rm -rf /tmp/repo
else
    echo "Pulling latest changes..."

    # Backup config files before pull
    if [ -f "./config/config.yml" ]; then
        cp ./config/config.yml ./config/config.yml.bak
        echo "Config backed up."
    fi
    if [ -f "./.env" ]; then
        cp ./.env ./.env.bak
    fi

    export GIT_TERMINAL_PROMPT=0
    ORIG_URL=$(git remote get-url origin)

    # Use GitHub token if available for private repo
    GITHUB_TOKEN=""
    if [ -f "./.env" ]; then
        GITHUB_TOKEN=$(grep -oP 'GITHUB_TOKEN=\K.*' ./.env 2>/dev/null || true)
    fi
    if [ -n "$GITHUB_TOKEN" ]; then
        git remote set-url origin "https://${GITHUB_TOKEN}@github.com/sang765/TMR-Discord-Bot.git"
    fi

    git fetch --all || echo "git fetch failed, continuing..."
    git reset --hard origin/main || echo "git reset failed, using local"

    # Restore original remote URL (không để token nằm trong .git/config)
    if [ -n "$GITHUB_TOKEN" ]; then
        git remote set-url origin "$ORIG_URL"
    fi

    # Restore config files after pull
    if [ -f "./config/config.yml.bak" ]; then
        mv ./config/config.yml.bak ./config/config.yml
        echo "Config restored."
    fi
    if [ -f "./.env.bak" ]; then
        mv ./.env.bak ./.env
    fi
fi

# ---------------------------------------------------------------
# 2) Cài Go (nếu thiếu) rồi build
#    Bot chỉ dùng Go stdlib + yaml — không cần ffmpeg hay gì khác
# ---------------------------------------------------------------
export GOROOT=/home/container/go
export GOPATH=/home/container/gopath
export GOMODCACHE=/home/container/gopath/pkg/mod
export GOTOOLCHAIN=local
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH

if [ ! -x "$GOROOT/bin/go" ]; then
    echo "Installing Go..."
    ARCH=$(uname -m)
    if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        GO_URL="https://go.dev/dl/go1.23.6.linux-arm64.tar.gz"
    else
        GO_URL="https://go.dev/dl/go1.23.6.linux-amd64.tar.gz"
    fi
    wget -q "$GO_URL" -O /tmp/go.tar.gz && tar -C /home/container -xzf /tmp/go.tar.gz && rm -f /tmp/go.tar.gz
fi

echo "Compiling..."
# Build ra file tạm: build fail thì giữ nguyên binary cũ, bot không bị chết giữa chừng
if CGO_ENABLED=0 go build -o tmr-bot.new .; then
    mv -f tmr-bot.new tmr-bot
    echo "Build successful!"
else
    echo "Build failed!"
    rm -f tmr-bot.new
    if [ ! -f "./tmr-bot" ]; then
        exit 1
    fi
    echo "Falling back to existing tmr-bot binary."
fi

# ---------------------------------------------------------------
# 3) (Tuỳ chọn) WARP proxy: nếu có wireproxy.conf thì mở SOCKS5 local
# ---------------------------------------------------------------
if [ -f "./wireproxy.conf" ]; then
    if [ ! -x "/home/container/wireproxy" ]; then
        echo "Installing wireproxy..."
        ARCH=$(uname -m)
        case "$ARCH" in
            aarch64|arm64) WP_URL="https://github.com/pufferffish/wireproxy/releases/latest/download/wireproxy_linux_arm64.tar.gz" ;;
            *) WP_URL="https://github.com/pufferffish/wireproxy/releases/latest/download/wireproxy_linux_amd64.tar.gz" ;;
        esac
        wget -q "$WP_URL" -O /tmp/wp.tar.gz \
            && tar -xzf /tmp/wp.tar.gz -C /home/container wireproxy \
            && chmod +x /home/container/wireproxy
        rm -f /tmp/wp.tar.gz
    fi
    if [ -x "/home/container/wireproxy" ]; then
        echo "Starting wireproxy (WARP SOCKS5 on 127.0.0.1:40000)..."
        nohup /home/container/wireproxy -c /home/container/wireproxy.conf > /home/container/wireproxy.log 2>&1 &
    fi
fi

# ---------------------------------------------------------------
# 4) Kiểm tra .env rồi chạy
# ---------------------------------------------------------------
if [ ! -f "./.env" ]; then
    echo ".env file not found!"
    echo "Create .env with:"
    echo "DISCORD_TOKEN=your_token"
    echo "GUILD_ID=your_server_id"
    exit 1
fi

mkdir -p data
exec ./tmr-bot

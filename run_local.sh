#!/bin/bash
# Chạy TMR bot local (trên máy/môi trường này) kèm WARP nếu có.
# Không cần root. Token nằm trong .env (không commit).
cd "$(dirname "$0")" || exit 1

# .env: DISCORD_TOKEN / GUILD_ID (file đã gitignore)
if [ ! -f .env ]; then
    echo "Thiếu .env (DISCORD_TOKEN=...)"; exit 1
fi

# Build nếu chưa có binary
if [ ! -x ./tmr-bot ]; then
    echo "Build tmr-bot..."
    go build -o tmr-bot . || exit 1
fi

# WARP SOCKS5 local (tuỳ chọn): nếu có wireproxy.conf + binary thì bật proxy
mkdir -p data /root/warp
if [ -x /root/warp/wireproxy ] && [ -f /root/warp/wireproxy.conf ]; then
    if ! curl -s -m 5 --socks5-hostname 127.0.0.1:40000 https://www.cloudflare.com/cdn-cgi/trace >/dev/null 2>&1; then
        echo "Bật wireproxy (WARP)..."
        nohup /root/warp/wireproxy -c /root/warp/wireproxy.conf >> /root/warp/wireproxy.log 2>&1 &
    fi
    # chỉ bật proxy cho scrape nếu WARP thực sự lên
    if curl -s -m 8 --socks5-hostname 127.0.0.1:40000 https://www.cloudflare.com/cdn-cgi/trace 2>/dev/null | grep -q "warp=on"; then
        export SCRAPE_PROXY="socks5://127.0.0.1:40000"
        echo "Scrape qua WARP."
    else
        echo "WARP chưa lên — scrape đi thẳng (jina fallback lo phần còn lại)."
    fi
fi

exec ./tmr-bot

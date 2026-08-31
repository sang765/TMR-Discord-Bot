#!/bin/bash

chmod +x /home/container/tmr-bot
chmod +x /home/container/run.sh

cd /home/container

if [ ! -d ".git" ]; then
    echo "Cloning from git..."
    git clone https://github.com/sang765/TMR-Discord-Bot.git /tmp/repo
    cp -r /tmp/repo/* /home/container/
    cp -r /tmp/repo/.* /home/container/ 2>/dev/null
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

    git fetch --all
    git reset --hard origin/main

    # Restore config files after pull
    if [ -f "./config/config.yml.bak" ]; then
        mv ./config/config.yml.bak ./config/config.yml
        echo "Config restored."
    fi
    if [ -f "./.env.bak" ]; then
        mv ./.env.bak ./.env
    fi
fi

if [ -f "./tmr-bot" ]; then
    rm -f ./tmr-bot
fi

if [ ! -f "./tmr-bot" ]; then
    echo "Compiling..."

    export GOROOT=/home/container/go
    export GOPATH=/home/container/gopath
    export GOMODCACHE=/home/container/gopath/pkg/mod
    export PATH=$GOROOT/bin:$GOPATH/bin:$PATH

    if [ ! -d "/home/container/go" ]; then
        echo "Installing Go..."
        mkdir -p /home/container/go
        wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz -O /tmp/go.tar.gz
        tar -C /home/container -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
    fi

    # Install ffmpeg if not present (extract only binary, not models)
    if ! command -v ffmpeg &> /dev/null && [ ! -f "/home/container/ffmpeg" ]; then
        echo "Installing ffmpeg..."
        wget -q https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz -O /tmp/ffmpeg.tar.xz
        tar -xf /tmp/ffmpeg.tar.xz --wildcards '*/ffmpeg' --strip-components=1 -C /home/container/
        tar -xf /tmp/ffmpeg.tar.xz --wildcards '*/ffprobe' --strip-components=1 -C /home/container/
        chmod +x /home/container/ffmpeg /home/container/ffprobe
        rm -f /tmp/ffmpeg.tar.xz
    fi

    export PATH=/home/container:$PATH
    CGO_ENABLED=0 go build -o tmr-bot .
    if [ $? -ne 0 ]; then
        echo "Build failed!"
        exit 1
    fi
    echo "Build successful!"
fi

if [ ! -f "./.env" ]; then
    echo ".env file not found!"
    echo "Create .env with:"
    echo "DISCORD_TOKEN=your_token"
    echo "GUILD_ID=your_server_id"
    exit 1
fi

./tmr-bot

#!/bin/bash

cd /home/container

if [ ! -f "./tmr-bot" ]; then
    echo "Binary not found. Cloning from git..."

    if [ ! -d ".git" ]; then
        git clone https://github.com/sang765/TMR-Discord-Bot.git /tmp/repo
        cp -r /tmp/repo/* /home/container/
        cp -r /tmp/repo/.* /home/container/ 2>/dev/null
        rm -rf /tmp/repo
    fi

    export GOROOT=/home/container/go
    export PATH=$GOROOT/bin:$PATH

    if [ ! -d "/home/container/go" ]; then
        echo "Installing Go..."
        mkdir -p /home/container/go
        wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz -O /tmp/go.tar.gz
        tar -C /home/container -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
    fi

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

chmod +x tmr-bot
./tmr-bot

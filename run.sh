#!/bin/bash

cd /home/container

if [ ! -f "./tmr-bot" ]; then
    echo "Binary not found. Compiling..."
    go build -o tmr-bot .
    if [ $? -ne 0 ]; then
        echo "Build failed!"
        exit 1
    fi
    echo "Build successful!"
fi

if [ ! -f "./.env" ]; then
    echo ".env file not found!"
    exit 1
fi

chmod +x tmr-bot
./tmr-bot

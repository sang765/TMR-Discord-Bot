#!/bin/bash

cd /home/container

if [ ! -f "./tmr-bot" ]; then
    echo "Binary not found. Please upload tmr-bot or compile with: go build -o tmr-bot ."
    exit 1
fi

if [ ! -f "./.env" ]; then
    echo ".env file not found!"
    exit 1
fi

./tmr-bot

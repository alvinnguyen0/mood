#!/bin/sh
set -e

cd ~/mood
git fetch origin main
git reset --hard origin/main
docker pull ghcr.io/alvinnguyen0/mood:latest
docker compose up -d

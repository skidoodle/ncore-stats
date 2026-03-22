# nCore Stats

A tracker for nCore profile statistics.

![nCore Stats Dashboard](https://github.com/user-attachments/assets/61fc64c6-c1ce-4d98-ade3-8a6ef3ee3f5e)

## Quick Start

1. Create a `.env` file with your credentials:

   ```ini
   NICK=your_nick
   PASS=your_password
   ```

2. Save this `compose.yaml`:

   ```yaml
   services:
     ncore-stats:
       image: ghcr.io/skidoodle/ncore-stats:latest
       container_name: ncore-stats
       restart: unless-stopped
       user: "1000:1000"
       ports:
         - "3000:3000"
       volumes:
         - data:/app/data
       configs:
         - source: users_config
           target: /app/users.txt
       environment:
         - NICK=${NICK}
         - PASS=${PASS}

   configs:
     users_config:
       content: |
         alice:123
         bob:456

   volumes:
     data:
   ```

3. Run `docker compose up -d`.

### How to get NICK and PASS

1. Log in to nCore in your browser using **"lower security"** mode.
2. Open Developer Tools (F12) and go to the **Network** tab.
3. Refresh, find any request to `ncore.pro`.
4. Check the **Cookie** request header for `nick=...; pass=...`.

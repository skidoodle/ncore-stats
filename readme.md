# nCore Profile Tracker

A simple Go project to scrape and track profile statistics (rank, upload, download, points) on nCore, the largest Hungarian BitTorrent tracker. The stats are displayed on a basic web interface and saved as JSON for historical tracking.

## Features

- Scrapes and logs profile stats from nCore.
- Serves a simple HTML dashboard to display the latest data.
- Provides a JSON API to fetch historical profile data.
- Automatically updates data every 24 hours.

## Setup

1. Clone the repo:

    ```bash
    git clone https://github.com/skidoodle/ncore-stats.git
    cd ncore-stats
    ```

2. Create a `.env` file with your nCore credentials:

    ```bash
    NICK=your_nick
    PASS=your_password
    ```

3. Add the `profiles.json` file, which contains your profile IDs:

    ```json
    {
      "alice": "69",
      "bob": "420"
    }
    ```

### How to obtain `NICK` and `PASS`

- Open the developer tools in your browser (F12), go to the "Network" tab.
- Log in using "lower security" mode.
- Find the `login.php` request in the network activity.
- In the response headers, locate the `Set-Cookie` header, which will contain `nick=` and `pass=` values.
- Copy those values and add them to your `.env` file.

## Running with Docker Compose

To deploy the project using Docker Compose:

1. Create the following `docker-compose.yml` file:

    ```yaml
    services:
      ncore-stats:
        image: ghcr.io/skidoodle/ncore-stats:main
        container_name: ncore-stats
        restart: unless-stopped
        ports:
          - "3000:3000"
        volumes:
          - data:/app/data

    volumes:
      data:
    ```

2. Run the Docker Compose setup:

    ```bash
    docker-compose up -d
    ```

3. Open `:3000` to view your stats.

### Updating

To pull the latest image and restart the service:

```bash
docker-compose pull
docker-compose up -d

# nCore Profile Tracker

A simple Go project to scrape and track profile statistics (rank, upload, download, points) on nCore, the largest Hungarian BitTorrent tracker. The stats are stored in a SQLite database and displayed on a basic web interface.

## Features

-   Scrapes and logs profile stats from nCore for multiple users.
-   Serves a simple HTML dashboard to display the latest data and historical charts.
-   Provides a JSON API to fetch historical profile data.
-   Stores all data persistently in a SQLite database.
-   Automatically updates data every 24 hours.

## Setup

1.  **Clone the repo:**

    ```bash
    git clone https://github.com/skidoodle/ncore-stats
    cd ncore-stats
    ```

2.  **Create a `.env` file with your nCore credentials:**

    ```ini
    NICK=your_nick
    PASS=your_password
    ```

3.  **Add Users to Track**

    You can add users via the `--add-user` command-line flag. Run this command for each user you want to track.

    The format is a single string: `'DisplayName,ProfileID'`.

    ```bash
    # Example for running from source
    go run . --add-user 'Alice,69'
    go run . --add-user 'Bob,420'
    ```

    > **How to find a Profile ID?**
    > Navigate to a user's profile on nCore. The URL will be `https://ncore.pro/profile.php?id=12345`. The `ProfileID` is the number at the end.

### How to obtain `NICK` and `PASS`

-   Open the developer tools in your browser (F12), go to the "Network" tab.
-   Log in to nCore using "lower security" mode.
-   Find the `login.php` request in the network activity.
-   In the response headers, locate the `Set-Cookie` header, which will contain `nick=` and `pass=` values.
-   Copy those values and add them to your `.env` file.

## Running with Docker Compose

1.  **Create the following `docker-compose.yml` file:**

    ```yaml
    services:
      ncore-stats:
        image: ghcr.io/skidoodle/ncore-stats:main
        container_name: ncore-stats
        restart: unless-stopped
        ports:
          - "3000:3000"
        volumes:
          - ./data:/app/data
        environment:
            - NICK=${NICK}
            - PASS=${PASS}
    ```

2.  **Add Users using Docker**

    Before starting the service, or to add new users later, run the `--add-user` command inside a temporary container. This ensures the user is added to the database in your persistent volume.

    ```bash
    # Use 'docker compose run' to add users before starting
    docker compose run --rm ncore-stats --add-user 'Alice,69'
    docker compose run --rm ncore-stats --add-user 'Bob,420'
    ```

    If the container is already running, you can use `docker exec`:
    ```bash
    # The executable inside the container is named 'ncore-stats'
    docker exec ncore-stats ./ncore-stats --add-user 'Charlie,1337'
    ```

3.  **Run the Docker Compose setup:**

    Once you have added your users, start the service.

    ```bash
    docker compose up -d
    ```

4.  Open `http://localhost:3000` to view your stats.

### Updating

To pull the latest image and restart the service:

```bash
docker compose pull
docker compose up -d
```

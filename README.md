# gator

RSS feed aggregator

### Requirements

- `Go` >= v1.25
- `Postgres` >= v16

### How to run

- run in dev mode: `go run . help`
- build: `go build` -> `./gator help`
- install: `go install` -> `gator help`

## Usage

### 1. Add config

Create config file (precedence):

- `.gatorconfig.json` (in current directory)
- `~/.gatorconfig.json` (in home directory)

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable"
}
```

### 2. Start DB

Start your own Postgres DB, or use provided `docker-compose` file:

```sh
docker compose up [-d]
```

### 3. Run DB migrations

```sh
gator dbmigrate
```

### 4. Start using

```sh
# show all commands
gator help

# create a user
gator register bob

# add feeds
gator addfeed HackerNews https://news.ycombinator.com/rss

# fetch posts for feeds
gator agg 1m

# show latest posts
gator browse
```

package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"gator/internal/config"
	"gator/internal/database"
	"gator/internal/rss"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

type state struct {
	config *config.Config
	db     *database.Queries
}

type command struct {
	name string
	args []string
}

type cmdHandlerFn func(*state, command) error

type commands struct {
	handlers map[string]cmdHandlerFn
}

func (c *commands) run(s *state, cmd command) error {
	f, ok := c.handlers[cmd.name]
	if !ok {
		return errors.New("command not found")
	}
	return f(s, cmd)
}

func (c *commands) register(name string, f cmdHandlerFn) {
	c.handlers[name] = f
}

//go:embed sql/schema/*.sql
var embedMigrations embed.FS

func runDBMigrations(db *sql.DB) cmdHandlerFn {
	return func(_ *state, _ command) error {
		goose.SetBaseFS(embedMigrations)

		if err := goose.SetDialect("postgres"); err != nil {
			return err
		}

		if err := goose.Up(db, "sql/schema"); err != nil {
			return err
		}

		return nil
	}
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) cmdHandlerFn {
	return func(s *state, c command) error {
		ctx := context.Background()

		user, err := s.db.GetUser(ctx, s.config.UserName)
		if err != nil {
			return fmt.Errorf("User not found: %w", err)
		}

		return handler(s, c, user)
	}
}

func handleLogin(s *state, cmd command) error {
	if len(cmd.args) != 1 {
		return errors.New("expected 1 argument: username")
	}
	username := cmd.args[0]

	ctx := context.Background()

	user, err := s.db.GetUser(ctx, username)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("user: %+v\n", user)

	if err := s.config.SetUser(username); err != nil {
		return err
	}

	fmt.Println("Set user to:", username)

	return nil
}

func handleRegister(s *state, cmd command) error {
	if len(cmd.args) != 1 {
		return errors.New("expected 1 argument: username")
	}
	username := cmd.args[0]

	ctx := context.Background()

	id := uuid.New()
	now := time.Now()

	user, err := s.db.CreateUser(ctx, database.CreateUserParams{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Name:      username,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("user created: %+v\n", user)

	if err = s.config.SetUser(username); err != nil {
		log.Fatal(err)
	}

	return nil
}

func handleReset(s *state, _ command) error {
	ctx := context.Background()
	return s.db.ResetDB(ctx)
}

func handleListUsers(s *state, _ command) error {
	ctx := context.Background()

	users, err := s.db.GetUsers(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, user := range users {
		current := ""
		if s.config.UserName == user {
			current = "(current)"
		}
		fmt.Println("*", user, current)
	}

	return nil
}

func scrapeFeeds(s *state) error {
	fmt.Println("Getting next feed ...")

	ctx := context.Background()

	feedRow, err := s.db.GetNextFeedToFetch(ctx)
	if err != nil {
		return fmt.Errorf("Can't get next feed to fetch: %w", err)
	}

	fmt.Printf("%+v\n", feedRow)

	_, err = s.db.MarkFeedFetched(ctx, database.MarkFeedFetchedParams{
		ID:            feedRow.ID,
		LastFetchedAt: sql.NullTime{Valid: true, Time: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("Error marking feed as fetched: %w", err)
	}

	feed, err := rss.FetchFeed(ctx, feedRow.Url)
	if err != nil {
		return fmt.Errorf("Failed to fetch feed: %w", err)
	}

	return saveFeedPostsToDB(feedRow.ID, feed, s.db)
}

func printFeed(feed *rss.RSSFeed) {
	fmt.Println(feed.Channel.Title)
	fmt.Println(feed.Channel.Link)
	fmt.Println(feed.Channel.Description)

	for i, item := range feed.Channel.Item[:5] {
		fmt.Printf("%d. %s", i+1, item.Title)
		// fmt.Printf("\n%s", item.Link)
		// fmt.Printf("\n%s", item.Description)
		fmt.Printf("\n")
	}
}

func parsePostPubTime(pubTime string) sql.NullTime {
	if pt, err := time.Parse(time.RFC1123Z, pubTime); err == nil {
		return sql.NullTime{Valid: true, Time: pt}
	}

	if pt, err := time.Parse(time.RFC1123, pubTime); err == nil {
		return sql.NullTime{Valid: true, Time: pt}
	}

	if pt, err := time.Parse(time.RFC3339, pubTime); err == nil {
		return sql.NullTime{Valid: true, Time: pt}
	}

	if pt, err := time.Parse(time.RubyDate, pubTime); err == nil {
		return sql.NullTime{Valid: true, Time: pt}
	}

	if pt, err := time.Parse(time.UnixDate, pubTime); err == nil {
		return sql.NullTime{Valid: true, Time: pt}
	}

	if pt, err := time.Parse(time.ANSIC, pubTime); err == nil {
		return sql.NullTime{Valid: true, Time: pt}
	}

	return sql.NullTime{}
}

func prettyPrint(i any) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func saveFeedPostsToDB(feedId uuid.UUID, feed *rss.RSSFeed, db *database.Queries) error {
	ctx := context.Background()
	now := time.Now()
	hasErrors := false

	for _, post := range feed.Channel.Item {
		fmt.Println()
		pubtime := parsePostPubTime(post.PubDate)
		fmt.Printf("%+v <- %s\n", pubtime, post.PubDate)

		params := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   now,
			UpdatedAt:   now,
			Title:       post.Title,
			Url:         post.Link,
			Description: sql.NullString{Valid: true, String: post.Description},
			PublishedAt: pubtime,
			FeedID:      feedId,
		}

		fmt.Printf("%s\n", prettyPrint(params))

		err := db.CreatePost(ctx, params)

		if err != nil {
			fmt.Printf("save to DB error: %s\n", err.Error())
			hasErrors = true
		}
	}

	if hasErrors {
		return errors.New("save to DB: has errors")
	}
	return nil
}

func handleAgg(s *state, c command) error {
	if len(c.args) != 1 {
		return errors.New("extected 1 arg: time-between-reqs")
	}

	d, err := time.ParseDuration(c.args[0])
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	fmt.Printf("Collecting feeds every %s\n", d)

	ticker := time.NewTicker(d)

	for ; ; <-ticker.C {
		err = scrapeFeeds(s)
		if err != nil {
			fmt.Printf("scrape error: %s\n", err.Error())
		}
	}
}

func handleAddFeed(s *state, c command, user database.User) error {
	if len(c.args) != 2 {
		return errors.New("expected 2 args: feed-name feed-url")
	}
	feedName := c.args[0]
	feedUrl := c.args[1]

	ctx := context.Background()

	now := time.Now()

	feed, err := s.db.CreateFeed(ctx, database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      feedName,
		Url:       feedUrl,
		UserID:    user.ID,
	})
	if err != nil {
		return err
	}

	now = time.Now()

	_, err = s.db.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", feed)
	return nil
}

func handleListAllFeeds(s *state, _ command) error {
	ctx := context.Background()

	feeds, err := s.db.GetAllFeeds(ctx)
	if err != nil {
		return err
	}

	for _, feed := range feeds {
		fmt.Println(" -", feed.FeedName, feed.Url, feed.UserName)
	}
	return nil
}

func handleFollow(s *state, c command, user database.User) error {
	if len(c.args) != 1 {
		return errors.New("expected 1 arg: feed-url")
	}
	feedUrl := c.args[0]

	ctx := context.Background()

	feed, err := s.db.GetFeedByUrl(ctx, feedUrl)
	if err != nil {
		return fmt.Errorf("feed not found: %w", err)
	}

	now := time.Now()

	_, err = s.db.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return err
	}

	fmt.Println(user.Name, feed.Name)
	return nil
}

func handleFollowing(s *state, _ command, user database.User) error {
	ctx := context.Background()

	follows, err := s.db.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return err
	}

	fmt.Println(user.Name, "following:")

	for _, feed := range follows {
		fmt.Println(" -", feed.Name, feed.Url)
	}

	return nil
}

func handleUnfollow(s *state, c command, user database.User) error {
	if len(c.args) != 1 {
		return errors.New("expected 1 arg: feed-url")
	}
	feedUrl := c.args[0]

	ctx := context.Background()

	feed, err := s.db.GetFeedByUrl(ctx, feedUrl)
	if err != nil {
		return err
	}

	err = s.db.DeleteFollow(ctx, database.DeleteFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	return err
}

func handleBrowse(s *state, c command, user database.User) error {
	var limit int32 = 2

	if len(c.args) > 1 {
		return errors.New("expected 1 arg or less: [limit]")
	}

	if len(c.args) == 1 {
		i, err := strconv.ParseInt(c.args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid limit: %w", err)
		}
		limit = int32(i)
	}

	ctx := context.Background()

	posts, err := s.db.GetPostsForUser(ctx, database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  limit,
	})
	if err != nil {
		return err
	}

	for i, post := range posts {
		fmt.Printf("\n%d: %s\n", i+1, prettyPrint(post))
	}

	return nil
}

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatal("command required")
	}

	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("postgres", cfg.DBUrl)
	if err != nil {
		log.Fatal(err)
	}

	dbQueries := database.New(db)

	curState := state{
		config: &cfg,
		db:     dbQueries,
	}

	cmds := commands{
		handlers: map[string]cmdHandlerFn{},
	}

	cmds.register("dbmigrate", runDBMigrations(db))

	cmds.register("reset", handleReset)
	cmds.register("agg", handleAgg)

	cmds.register("login", handleLogin)
	cmds.register("register", handleRegister)

	cmds.register("users", handleListUsers)
	cmds.register("feeds", handleListAllFeeds)

	cmds.register("addfeed", middlewareLoggedIn(handleAddFeed))
	cmds.register("follow", middlewareLoggedIn(handleFollow))
	cmds.register("following", middlewareLoggedIn(handleFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handleUnfollow))
	cmds.register("browse", middlewareLoggedIn(handleBrowse))

	err = cmds.run(&curState, command{name: args[1], args: args[2:]})
	if err != nil {
		log.Fatal(err)
	}
}

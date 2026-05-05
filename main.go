package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gator/internal/config"
	"gator/internal/database"
	"gator/internal/rss"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
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
	return s.db.Reset(ctx)
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

func handleAgg(s *state, _ command) error {
	url := "https://www.wagslane.dev/index.xml"

	ctx := context.Background()

	feed, err := rss.FetchFeed(ctx, url)
	if err != nil {
		return err
	}

	fmt.Println(feed.Channel.Title)
	fmt.Println(feed.Channel.Link)
	fmt.Println(feed.Channel.Description)

	for i, item := range feed.Channel.Item {
		fmt.Printf(
			"\n%d. %s\n%s\n%s\n",
			i+1, item.Title, item.Link, item.Description,
		)
	}

	fmt.Printf("%+v\n", *feed)
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

	cmds.register("login", handleLogin)
	cmds.register("register", handleRegister)
	cmds.register("reset", handleReset)
	cmds.register("users", handleListUsers)
	cmds.register("agg", handleAgg)

	err = cmds.run(&curState, command{name: args[1], args: args[2:]})
	if err != nil {
		log.Fatal(err)
	}
}

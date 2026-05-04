package main

import (
	"errors"
	"fmt"
	"gator/internal/config"
	"log"
	"os"
)

type state struct {
	config *config.Config
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
	if err := s.config.SetUser(username); err != nil {
		return err
	}
	fmt.Println("Set user to:", username)
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

	curState := state{config: &cfg}

	cmds := commands{handlers: map[string]cmdHandlerFn{}}

	cmds.register("login", handleLogin)

	err = cmds.run(&curState, command{name: args[1], args: args[2:]})
	if err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"fmt"
	"gator/internal/config"
	"log"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg)

	cfg.SetUser("lane")

	cfg, err = config.Read()
	if err != nil {
		log.Fatal(cfg)
	}

	fmt.Printf("%+v\n", cfg)
}

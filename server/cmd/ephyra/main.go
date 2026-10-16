package main

import (
	"log"

	"github.com/jellyfish-p/ephyra/server/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/himidori/starwars/starwars"
)

func main() {
	fetcher := starwars.NewFetcher()
	fetcher.Start()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	signal.Stop(ch)

	fetcher.Stop()
}

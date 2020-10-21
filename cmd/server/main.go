package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	hio "github.com/sshh12/hermes/io"
)

func main() {

	logLevel := flag.String("log", "debug", "Log level")
	flag.Parse()

	loggingLevel, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(loggingLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})

	server, err := hio.NewServer(4000)
	if err != nil {
		log.Fatal(err)
	}
	err = server.Start()
	if err != nil {
		log.Fatal(err)
	}

}

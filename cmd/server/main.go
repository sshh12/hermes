package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	hio "github.com/sshh12/hermes/io"
)

func main() {

	hermesPort := flag.Int("port", 4000, "Hermes server port")
	logLevel := flag.String("log", "info", "Log level")
	flag.Parse()

	loggingLevel, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(loggingLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})

	server, err := hio.NewServer(*hermesPort)
	if err != nil {
		log.Fatal(err)
	}
	err = server.Start()
	if err != nil {
		log.Fatal(err)
	}

}

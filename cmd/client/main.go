package main

import (
	"flag"
	"math/rand"
	"os"

	log "github.com/sirupsen/logrus"

	hio "github.com/sshh12/hermes/io"
)

func main() {

	localPort := flag.Int("port", 8080, "Local port to forward")
	remotePort := flag.Int("rport", 0, "Remote port (0 is random)")
	hermesPort := flag.Int("hport", 4000, "Hermes server port")
	hermesHost := flag.String("hhost", "localhost", "Address of hermes server")
	logLevel := flag.String("log", "info", "Log level")
	flag.Parse()

	if *hermesHost == "localhost" && os.Getenv("HERMES_HOST") != "" {
		*hermesHost = os.Getenv("HERMES_HOST")
	}

	loggingLevel, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(loggingLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})

	for *remotePort <= 0 || *remotePort == *hermesPort {
		*remotePort = rand.Intn(10000) + 4001
	}

	client, err := hio.NewClient(*localPort, *remotePort, *hermesPort, *hermesHost)
	if err != nil {
		log.Fatal(err)
	}
	err = client.Start()
	if err != nil {
		log.Fatal(err)
	}

}

package main

import (
	"crypto/tls"
	"flag"

	log "github.com/sirupsen/logrus"

	tcp "github.com/sshh12/hermes/tcp"
)

func main() {

	port := flag.Int("port", 4000, "Hermes server port")
	tlsPort := flag.Int("tls_port", 4001, "Hermes TLS server port")
	tlsCert := flag.String("tls_crt", "server.crt", "Hermes TLS .crt")
	tlsKey := flag.String("tls_key", "server.key", "Hermes TLS .key")
	logLevel := flag.String("log", "info", "Log level")
	flag.Parse()

	loggingLevel, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(loggingLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})

	options := make([]tcp.ServerOption, 0)

	cer, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
	if err != nil {
		log.Warning("Failed to find or load TLS certificate")
	} else {
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cer}}
		options = append(options, tcp.WithServerTLS(*tlsPort, tlsCfg))
	}

	server, err := tcp.NewServer(*port, options...)
	if err != nil {
		log.Fatal(err)
	}
	err = server.Start()
	if err != nil {
		log.Fatal(err)
	}

}

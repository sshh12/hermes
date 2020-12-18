package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	log "github.com/sirupsen/logrus"
	tcp "github.com/sshh12/hermes/tcp"
)

type clientConfig struct {
	path                string
	HermesHost          string `json:"host"`
	HermesPort          int    `json:"port"`
	HermesTLSPort       int    `json:"tls_port"`
	UseTLS              bool   `json:"use_tls"`
	TLSIgnoreSkipVerify bool   `json:"tls_ignore_skip_verify"`
}

func newConfig() (*clientConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	hermesPath := path.Join(home, ".hermes")
	os.MkdirAll(hermesPath, os.ModePerm)
	return &clientConfig{
		path: path.Join(hermesPath, "config.json"),
	}, nil
}

func (cfg *clientConfig) Read() error {
	if _, err := os.Stat(cfg.path); os.IsNotExist(err) {
		cfg.HermesHost = "127.0.0.1"
		cfg.HermesPort = 4000
		cfg.HermesTLSPort = 4001
		cfg.UseTLS = false
		cfg.TLSIgnoreSkipVerify = false
		return nil
	}
	encoded, err := ioutil.ReadFile(cfg.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, cfg)
}

func (cfg *clientConfig) Write() error {
	encoded, _ := json.MarshalIndent(cfg, "", " ")
	return ioutil.WriteFile(cfg.path, encoded, 0644)
}

func main() {

	cfg, err := newConfig()
	if err != nil {
		log.Fatal(err)
	}
	cfg.Read()

	var noSpin bool
	flag.IntVar(&cfg.HermesPort, "port", cfg.HermesPort, "Hermes server port")
	flag.IntVar(&cfg.HermesTLSPort, "tls_port", cfg.HermesTLSPort, "Hermes server port")
	flag.BoolVar(&cfg.UseTLS, "use_tls", cfg.UseTLS, "Use TLS")
	flag.BoolVar(&noSpin, "no_spin", false, "Don't display cool spinner thing")
	flag.BoolVar(&cfg.TLSIgnoreSkipVerify, "tls_skip_verify", cfg.TLSIgnoreSkipVerify, "Don't attempted to verify hermes server TLS cert")
	flag.StringVar(&cfg.HermesHost, "server", cfg.HermesHost, "Address of hermes server")
	save := flag.Bool("save", false, "Set these settings as defaults")
	logLevel := flag.String("log", "error", "Log level")
	flag.Parse()
	args := flag.Args()

	loggingLevel, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(loggingLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})

	if *save {
		log.Info("Saving settings")
		if err := cfg.Write(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if len(args) == 0 {
		log.Fatal("No port to forward, do $ hermes <port>")
		return
	}

	wg := &sync.WaitGroup{}
	disp := make([]string, 0)
	for i := 0; i < len(args); i += 2 {

		appPort, err := strconv.Atoi(args[i])
		if err != nil {
			log.Fatal(err)
			return
		}
		var remotePort int
		if i+1 < len(args) {
			remotePort, err = strconv.Atoi(args[i+1])
			if err != nil {
				log.Fatal(err)
				return
			}
		} else {
			remotePort = appPort
		}
		wg.Add(1)
		disp = append(disp, fmt.Sprintf("localhost:%d <-> %s:%d", appPort, cfg.HermesHost, remotePort))

		options := []tcp.ClientOption{
			tcp.WithRestarts(),
		}

		if cfg.UseTLS {
			serverTLSAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", cfg.HermesHost, cfg.HermesTLSPort))
			if err != nil {
				log.Fatal("Invalid server address")
				return
			}
			tlsConf := &tls.Config{
				InsecureSkipVerify: true,
			}
			options = append(options, tcp.WithTLS(serverTLSAddr, tlsConf))
		} else {
			serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", cfg.HermesHost, cfg.HermesPort))
			if err != nil {
				log.Fatal("Invalid server address")
				return
			}
			options = append(options, tcp.WithServerAddress(serverAddr))
		}

		go func() {
			client, err := tcp.NewClient(appPort, remotePort, cfg.HermesHost, options...)
			if err != nil {
				log.Error(err)
			}
			if err := client.Start(); err != nil {
				log.Error(err)
			}
			wg.Done()
		}()

	}
	var displayText string
	if cfg.UseTLS {
		displayText = "Forwarding (🔒) " + strings.Join(disp, ", ")
	} else {
		displayText = "Forwarding " + strings.Join(disp, ", ")
	}
	if !noSpin {
		spin := spinner.New(spinner.CharSets[11], 500*time.Millisecond, spinner.WithWriter(os.Stderr), spinner.WithHiddenCursor(true))
		spin.Suffix = " " + displayText
		spin.Start()
		wg.Wait()
		spin.Stop()
	} else {
		fmt.Println(displayText)
		wg.Wait()
	}

}

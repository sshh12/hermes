package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	log "github.com/sirupsen/logrus"
	hio "github.com/sshh12/hermes/io"
)

type clientConfig struct {
	path       string
	HermesHost string `json:"hhost"`
	HermesPort int    `json:"hport"`
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

	flag.IntVar(&cfg.HermesPort, "hport", cfg.HermesPort, "Hermes server port")
	flag.StringVar(&cfg.HermesHost, "hhost", cfg.HermesHost, "Address of hermes server")
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
	}

	if len(args) == 0 {
		log.Fatal("No port to forward, do ./hermes <port>")
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
		go func() {
			client, err := hio.NewClient(appPort, remotePort, cfg.HermesPort, cfg.HermesHost)
			if err != nil {
				log.Error(err)
			}
			if err := client.Start(); err != nil {
				log.Error(err)
			}
			wg.Done()
		}()
	}
	spin := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
	spin.Suffix = " Forwarding " + strings.Join(disp, ", ")
	spin.Start()
	wg.Wait()

}

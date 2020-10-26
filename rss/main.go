package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/aldor/something/rss/srv"
)

type serverFailed struct {
	err error
}

func (s serverFailed) String() string {
	return fmt.Sprintf("server failed: %s", s.err)
}
func (s serverFailed) Signal() {}

func main() {
	configPath := flag.String("config", "", "path to service config")
	dataPath := flag.String("data", "", "path to boltdb data file")
	flag.Parse()
	if *configPath == "" || *dataPath == "" {
		flag.Usage()
		os.Exit(1)
	}
	data, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("read config %s: %s", *configPath, err)
	}
	config := srv.Config{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("parse config %s: %s", *configPath, err)
	}
	app, err := srv.NewApp(config, *dataPath)
	if err != nil {
		log.Fatalf("creating new app: %s", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Serve(":8080"); err != nil {
			log.Errorf("app server: %s", err)
			stop <- serverFailed{err}
		}
	}()

	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	app.Shutdown(ctx)
}

package main

import (
	"encoding/json"
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
)

func main() {
	// Resolving command line parameters
	configFilePath := flag.String("c", "agent_conf.json", "Path of config file")
	logDir := flag.String("l", "", "Log directory")
	flag.Parse()

	if *logDir == "" || *logDir == "stderr" {
		flag.Lookup("logtostderr").Value.Set("true")
	} else {
		flag.Lookup("log_dir").Value.Set(*logDir)
	}

	// Increase file descriptor
	IncreaseFDLimit()

	// Read configuration file
	config := NewConfig()
	err := config.LoadFromFile(*configFilePath)
	if err != nil {
		glog.Fatal("load config failed: ", err)
		return
	}
	config.Init()

	// Print loaded profile (for debugging)
	if glog.V(3) {
		configBytes, _ := json.Marshal(config)
		glog.Info("config: ", string(configBytes))
	}

	// Start HTTP debugging service
	if config.HTTPDebug.Enable {
		glog.Info("HTTP debug enabled: ", config.HTTPDebug.Listen)
		go func() {
			err := http.ListenAndServe(config.HTTPDebug.Listen, nil)
			if err != nil {
				glog.Error("launch http debug service failed: ", err.Error())
			}
		}()
	}

	// Session manager
	manager := NewSessionManager(config)

	// Exit signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		glog.Info("exiting...")
		manager.Stop()
	}()

	// Run agency
	manager.Run()
}

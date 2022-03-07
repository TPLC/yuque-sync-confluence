package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"yuque-sync-confluence/config"
	"yuque-sync-confluence/internal/converter"
	"yuque-sync-confluence/internal/notification"
)

func loadConfig(filename string, v interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func getEnv(key string) (string, error) {
	val, exist := os.LookupEnv(key)
	if !exist {
		return "", errors.New(fmt.Sprintf("env %v not exist", key))
	}

	return val, nil
}

func main() {
	cfgLocation, err := getEnv("USERCONF")
	if err != nil {
		log.Fatal(err)
	}

	cfgFileName := ""
	if runtime.GOOS == "windows" {
		cfgFileName = cfgLocation + "\\yuque-sync-confluence.json"
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		cfgFileName = cfgLocation + "/yuque-sync-confluence.json"
	}

	cfg := &config.Config{}
	if err := loadConfig(cfgFileName, cfg); err != nil {
		log.Fatal(err)
	}

	n := notification.NewClient(cfg.Notification)

	docConverter, err := converter.NewConverter(cfg)
	if err != nil {
		n.Notify(err)
		log.Fatal(err)
	}
	if err := docConverter.Execute(); err != nil {
		n.Notify(err)
		log.Fatal(err)
	}

	n.Notify(nil)
	log.Println("success")
}

package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	TelegramToken string `yaml:"telegram_token"`
	GithubInfo    struct {
		Token  string `yaml:"token"`
		Owner  string `yaml:"owner"`
		Repo   string `yaml:"repo"`
		Branch string `yaml:"branch"`
	} `yaml:"github"`
	ExtensionsToLookFor []string  `yaml:"necessary_extensions"`
	Subjects            []Subject `yaml:"subjects"`
}

type Subject struct {
	Name []string `yaml:"subject"`
}

const configPath = "config.yml"

var Cfg AppConfig

func ReadConfig() {
	f, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&Cfg)

	if err != nil {
		panic(err)
	}
}

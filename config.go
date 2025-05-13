package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WebHooks []*WebHook `yaml:"webhooks"`
}

type WebHook struct {
	Path     string `yaml:"path"`
	Method   string `yaml:"method"`
	Response struct {
		Headers []Header `yaml:"headers"`
		Body    string   `yaml:"body"`
		Code    int      `yaml:"code"`
	} `yaml:"response"`
	Targets []string `yaml:"targets"`
}

type Header struct {
	Name         string `yaml:"name"`
	Value        string `yaml:"value"`
	ValueFromEnv string `yaml:"valueFromEnv"`
}

func (h Header) GetValue() string {
	if h.Value != "" {
		return h.Value
	}
	return os.Getenv(h.ValueFromEnv)
}

func loadConfig(configPath string) ([]*WebHook, error) {
	var config Config

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config.WebHooks, validateConfig(config)
}

func validateConfig(config Config) error {
	// all webhook's paths must be unique
	paths := make(map[string]bool)
	for _, wh := range config.WebHooks {
		if wh.Path == "" {
			return fmt.Errorf("empty path not allowed webhook:%s", wh.Path)
		}
		if !strings.HasPrefix(wh.Path, "/") {
			return fmt.Errorf("path should have '/' prefix webhook:%s", wh.Path)
		}
		if _, ok := paths[wh.Path]; ok {
			return fmt.Errorf("webhooks path must be unique duplicate found path:%s", wh.Path)
		}
		paths[wh.Path] = true
	}
	return nil
}

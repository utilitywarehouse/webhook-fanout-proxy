package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WebHooks []*WebHook `yaml:"webhooks"`
}

type WebHook struct {
	Path      string     `yaml:"path"`
	Method    string     `yaml:"method"`
	Signature *Signature `yaml:"signature"`
	Response  struct {
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

type Signature struct {
	HeaderName    string `yaml:"headerName"`
	Alg           string `yaml:"alg"`
	Prefix        string `yaml:"prefix"`
	SecretFromEnv string `yaml:"secretFromEnv"`
}

func (s *Signature) verify(message []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// select hash function
	var hashFunc func() hash.Hash
	switch {
	case strings.EqualFold(s.Alg, "sha1"):
		hashFunc = sha1.New
	default:
		hashFunc = sha256.New
	}

	hash := computeHMAC(hashFunc, message, os.Getenv(s.SecretFromEnv))

	return hmac.Equal([]byte(signature), []byte(s.Prefix+hash))
}

func computeHMAC(hashFunc func() hash.Hash, message []byte, secret string) string {

	mac := hmac.New(hashFunc, []byte(secret))
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
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

		if wh.Signature != nil {
			if wh.Signature.HeaderName == "" || wh.Signature.SecretFromEnv == "" {
				return fmt.Errorf("signature's header and secret env name is required :%v", wh.Signature)
			}

			if wh.Signature.Alg != "" && wh.Signature.Alg != "sha256" {
				return fmt.Errorf("signature: only sha256 alg without timestamp check is supported:%v", wh.Signature)
			}

			if os.Getenv(wh.Signature.SecretFromEnv) == "" {
				return fmt.Errorf("signature: secret env is not env:%s", wh.Signature.SecretFromEnv)

			}
		}

	}
	return nil
}

package config

import (
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"os"
)

type (
	Config struct {
		IntervalS       int    `json:"intervalS"`
		SecretKey       string `json:"secretKey"`
		SecretId        string `json:"secretId"`
		ModifyAtStartup bool   `json:"modifyAtStartup"`

		Domain     string `json:"domain"`
		SubDomain  string `json:"subDomain"`
		RecordId   int    `json:"recordId"`
		RecordLine string `json:"recordLine"`
		Ttl        int    `json:"ttl"`
	}
)

func Get() (*Config, error) {
	var (
		configFilePath *string
		file           *os.File
		bytes          []byte
		cfg            = &Config{}
	)

	configFilePath = flag.String("c", "./config.json", "config JSON file")
	flag.Parse()
	slog.Info("config file path", slog.String("file-path", *configFilePath))

	if f, err := os.Open(*configFilePath); err != nil {
		slog.Error("open config file error", slog.String("error", err.Error()))
		return nil, err
	} else {
		file = f
		defer func() {
			if file != nil {
				if e := file.Close(); e != nil {
					slog.Error("close config file error", slog.String("error", e.Error()))
				}
			}
		}()
	}

	if b, err := io.ReadAll(file); err != nil {
		slog.Error("read config file error", slog.String("error", err.Error()))
		return nil, err
	} else {
		bytes = b
	}

	if err := json.Unmarshal(bytes, cfg); err != nil {
		slog.Error("unmarshal config file error", slog.String("error", err.Error()))
		return nil, err
	}

	return cfg, nil
}

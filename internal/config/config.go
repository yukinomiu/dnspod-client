package config

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

type (
	Config struct {
		IntervalS       int    `json:"intervalS"`
		SecretKey       string `json:"secretKey"`
		SecretId        string `json:"secretId"`
		UpdateAtStartup bool   `json:"updateAtStartup"`

		Domain     string `json:"domain"`
		SubDomain  string `json:"subDomain"`
		RecordId   int    `json:"recordId"`
		RecordLine string `json:"recordLine"`
		Ttl        int    `json:"ttl"`

		ExternalPublicIPGetter struct {
			Enabled bool     `json:"enabled"`
			URLs    []string `json:"urls"`
		} `json:"externalPublicIPGetter"`
	}
)

func (c *Config) Validate() (err error) {
	defer func() {
		if err != nil {
			slog.Error(
				"bad config",
				slog.String("error", err.Error()),
			)
		}
	}()

	if c.IntervalS <= 0 {
		err = errors.New("interval seconds must be greater than zero")
		return
	}

	if c.SecretKey == "" {
		err = errors.New("secret key is required")
		return
	}

	if c.SecretId == "" {
		err = errors.New("secret ID is required")
		return
	}

	if c.Domain == "" {
		err = errors.New("domain is required")
		return
	}

	if c.SubDomain == "" {
		err = errors.New("sub domain is required")
		return
	}

	if c.RecordId <= 0 {
		err = errors.New("record ID must be greater than zero")
		return
	}

	if c.RecordLine == "" {
		err = errors.New("record line is required")
		return
	}

	if c.Ttl <= 0 {
		err = errors.New("ttl must be greater than zero")
		return
	}

	if c.ExternalPublicIPGetter.Enabled {
		if len(c.ExternalPublicIPGetter.URLs) == 0 {
			err = errors.New("external public IP getter's URL list is required")
			return
		}

		for _, urlStr := range c.ExternalPublicIPGetter.URLs {
			if urlStr == "" {
				err = errors.New("external public IP getter's URL can not be empty")
				return
			} else {
				if u, e := url.Parse(urlStr); e != nil {
					err = errors.New("external public IP getter's URL is invalid")
					return
				} else {
					s := strings.ToLower(u.Scheme)
					if s != "http" && s != "https" {
						err = errors.New("external public IP getter's URL scheme is invalid")
						return
					}
				}
			}
		}
	}

	return
}

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

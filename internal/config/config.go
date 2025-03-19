package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
    filePath        string
    CurrentUserName string  `json:"current_user_name"`
    DbUrl           string  `json:"db_url"`
}

func Read() (*Config, error) {
    // read from file frist
    path, err := getConfigFilePath()
    if err != nil {
        return nil, err
    }
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // now decode the result into the config
    cfg := &Config{}
    decoder := json.NewDecoder(bytes.NewReader(content))
    if err := decoder.Decode(cfg); err != nil {
        return nil, err
    }

    return cfg, nil
}

func getConfigFilePath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }

    path := fmt.Sprintf("%v/%v", home, configFileName)
    return path, nil
}

func (cfg *Config)SetUser(userName string) error {
    cfg.CurrentUserName = userName
    return write(*cfg)
}

func write(cfg Config) error {
    data, err := json.Marshal(&cfg)
    if err != nil {
        return err
    }
    path, err := getConfigFilePath()
    if err != nil {
        return err
    }

    return os.WriteFile(path, data, os.ModePerm)
}

const configFileName = ".gatorconfig.json"


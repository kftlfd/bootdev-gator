package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
)

const configFileName = ".gatorconfig.json"

var configFilePath = ""

type Config struct {
	DBUrl    string `json:"db_url"`
	UserName string `json:"current_user_name"`
}

func Read() (Config, error) {
	cfgPath, err := getConfigFilePath()
	if err != nil {
		return Config{}, err
	}

	configFilePath = cfgPath
	fmt.Println("Using config file:", configFilePath)

	cfgContents, err := os.ReadFile(cfgPath)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{}
	if err = json.Unmarshal(cfgContents, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) SetUser(userName string) error {
	c.UserName = userName

	return write(*c)
}

func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

func getConfigFilePath() (string, error) {
	curDir, err := os.Getwd()
	if err == nil {
		curDirConfigPath := path.Join(curDir, configFileName)
		if isRegularFile(curDirConfigPath) {
			return curDirConfigPath, nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		homeDirConfigPath := path.Join(homeDir, configFileName)
		if isRegularFile(homeDirConfigPath) {
			return homeDirConfigPath, nil
		}
	}

	return "", errors.New("no config file found")
}

func write(cfg Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configFilePath, data, 0644)
}

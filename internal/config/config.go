package config

import (
	"encoding/json"
	"io/ioutil"
)

//Config - структура для конфигурации
type Config struct {
	MaxDepth       int
	MaxResults     int
	MaxErrors      int
	Url            string
	RequestTimeout int //in seconds
	AppTimeout     int //in seconds

	ServerName     string
	LogLevelString string
	LogLevel       int
}

func NewConfig(path string) (Config, error) {
	data := Config{}
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return data, err
	}
	err = json.Unmarshal([]byte(file), &data)

	if err == nil {
		data.LogLevel = logLevelFromStringToInt(data.LogLevelString)
	}

	return data, err
}

var LogLevels = map[string]int{
	"Debug":  -1,
	"Info":   0,
	"Warn":   1,
	"Error":  2,
	"DPanic": 3,
	"Panic":  4,
	"Fatal":  5,
}

func logLevelFromStringToInt(logLevelName string) int {
	val, ok := LogLevels[logLevelName]
	if !ok {
		return 0
	}

	return val
}

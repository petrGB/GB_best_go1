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
}

func NewConfig(path string) (Config, error) {
	data := Config{}
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return data, err
	}
	err = json.Unmarshal([]byte(file), &data)
	return data, err
}

package main

type Config []ConfigItem

type ConfigItem struct {
	Listen string   `json:"listen"`
	To     string   `json:"to"`
	Accept []string `json:"accept"`
}

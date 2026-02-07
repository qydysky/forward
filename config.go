package main

type Config []ConfigItem

type ConfigItem struct {
	Listen  string   `json:"listen"`
	To      string   `json:"to"`
	IdleDru string   `json:"idleDru"`
	Accept  []string `json:"accept"`
}

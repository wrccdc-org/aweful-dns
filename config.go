package main

import (
	"encoding/json"
	"flag"
	"os"
)

type Config struct {
	DNSConfigs      map[string]interface{}
	CacheExpiration int64
	UseOutbound     bool
	LogLevel        string
}

func InitConfig() (string, bool, string, string, error) {
	logLevel := flag.String("log-level", "info", "log level")
	useOutbound := flag.Bool("use-outbound", false, "use outbound address")
	configDir := flag.String("config-dir", "/root/aweful-dns-confs", "directory containing config files")
	dnsmasqConfig := flag.String("dnsmasq-file", "/etc/dnsmasq.d/aweful-dns.conf", "dnsmasq config file to write (this will be overwritten)")
	flag.Parse()

	err := clearDnsmasqConfig(*dnsmasqConfig)
	if err != nil {
		return "", false, "", "", err
	}

	return *logLevel, *useOutbound, *configDir, *dnsmasqConfig, nil
}

func clearDnsmasqConfig(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("#Aweful dnsmasq config file\n#Warning: this file will be overwritten\n")
	if err != nil {
		return err
	}

	return nil
}

func ParseFile(filePath string) (map[string]interface{}, error) {
	fileContents := make(map[string]interface{})
	body, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &fileContents); err != nil {
		return nil, err
	}

	return fileContents, nil
}

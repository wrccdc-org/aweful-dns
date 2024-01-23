package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"strings"
)

func main() {
	logLevel, useOutbound, configDir, dnsmasqConfig, err := InitConfig()
	if err != nil {
		log.Fatalf("Failed to load configs: %s", err)
	}

	files, err := os.ReadDir(configDir)
	if err != nil {
		log.Fatalf("Failed to read config directory: %s", err)
	}

	for i, f := range files {
		lastIP := net.ParseIP(fmt.Sprintf("127.0.%d.0", i+1))

		if !strings.HasSuffix(f.Name(), ".json") {
			log.Printf("Not parsing file %s", f.Name())
			continue
		}
		configFile := path.Join(configDir, f.Name())
		dnsConfigs, err := ParseFile(configFile)
		if err != nil {
			log.Printf("Failed to load config from file %s: %s", configFile, err)
			return
		}

		log.Printf("Loaded config from %s", configFile)
		// Default to 8 teams
		teamCount := 8
		if numberTeams, ok := dnsConfigs["number_of_teams"].(float64); ok {
			teamCount = int(numberTeams)
		}

		for teamNum := 1; teamNum < teamCount+1; teamNum++ {
			log.Printf("Starting server for team %d", teamNum)
			ip, err := findAvailableLocalhostIP(lastIP)
			if err != nil {
				log.Fatalf("Failed to find available IP: %s", err)
				return
			}
			lastIP = ip

			go func(dnsConfigs map[string]interface{}, tn int, ip string, dnsmasqConfig string) {
				dnsConfigs["host"] = ip
				config := Config{
					DNSConfigs:  dnsConfigs,
					UseOutbound: useOutbound,
					LogLevel:    logLevel,
				}
				StartDNSServer(config, tn, dnsmasqConfig)
			}(dnsConfigs, teamNum, ip.String(), dnsmasqConfig)
		}
	}

	// Prevent the main goroutine from exiting
	select {}
}

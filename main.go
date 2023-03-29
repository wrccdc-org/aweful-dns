package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/miekg/dns"
)

func main() {
	appConfigs, err := InitConfig()
	if err != nil {
		log.Fatalf("Failed to load configs: %s", err)
	}

	dnsCache := InitCache(appConfigs.CacheExpiration)
	domains, _ := appConfigs.DNSConfigs["domains"]
	servers, _ := appConfigs.DNSConfigs["servers"]

	dnsProxy := DNSProxy{
		Cache:         &dnsCache,
		domains:       domains.(map[string]interface{}),
		servers:       servers.(map[string]interface{}),
		defaultServer: appConfigs.DNSConfigs["defaultDns"].(string),
	}

	logger := NewLogger(appConfigs.LogLevel)
	host := ""
	if appConfigs.UseOutbound {
		ip, err := GetOutboundIP()
		if err != nil {
			logger.Fatalf("Failed to get outbound address: %s\n ", err.Error())
		}
		host = ip.String() + ":53"
	} else {
		host = appConfigs.DNSConfigs["host"].(string)
	}
	var internal_mask string
	var external_mask string
	var mask_proxy_server string
	if appConfigs.DNSConfigs["mask_proxy_server"] == nil {
		mask_proxy_server = "http://checkip.amazonaws.com"
	} else {
		mask_proxy_server = appConfigs.DNSConfigs["mask_proxy_server"].(string)
	}
	if appConfigs.DNSConfigs["internal_mask"] == nil {
		ip, err := GetOutboundIP()
		if err != nil {
			logger.Fatalf("Failed to get outbound address: %s\n ", err.Error())
		}
		internal_mask = strings.Join(strings.Split(ip.String(), ".")[0:3], ".") + "."
	} else {
		internal_mask = appConfigs.DNSConfigs["internal_mask"].(string)
	}

	if appConfigs.DNSConfigs["external_mask"] == nil {
		resp, err := http.Get(mask_proxy_server)
		if err != nil {
			logger.Fatalf("idk some error %s\n", err.Error())
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		ip_string := fmt.Sprintf("%s", body)
		external_mask = strings.Join(strings.Split(ip_string, ".")[0:3], ".") + "."
	} else {
		external_mask = appConfigs.DNSConfigs["external_mask"].(string)
	}

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		switch r.Opcode {
		case dns.OpcodeQuery:
			m, err := dnsProxy.getResponse(r, internal_mask, external_mask)
			if err != nil {
				logger.Errorf("Failed lookup for %s with error: %s\n", r, err.Error())
				m.SetReply(r)
				w.WriteMsg(m)
				return
			}
			if len(m.Answer) > 0 {
				pattern := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
				ipAddress := pattern.FindAllString(m.Answer[0].String(), -1)

				if len(ipAddress) > 0 {
					logger.Infof("Lookup for %s with ip %s\n", m.Answer[0].Header().Name, ipAddress[0])
				} else {
					logger.Infof("Lookup for %s with response %s\n", m.Answer[0].Header().Name, m.Answer[0])
				}
			}
			m.SetReply(r)
			w.WriteMsg(m)
		}
	})

	server := &dns.Server{Addr: host, Net: "udp"}
	logger.Infof("Starting at %s\n", host)
	err = server.ListenAndServe()
	if err != nil {
		logger.Errorf("Failed to start server: %s\n ", err.Error())
	}
}

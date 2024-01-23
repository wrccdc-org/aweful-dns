package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/miekg/dns"
)

// This assumes that each team has the same IP address internally and their external IP address is configured by team number
func StartDNSServer(appConfigs Config, teamNum int, dnsmasqConfig string) {
	logger := NewLogger(appConfigs.LogLevel)
	host := ""
	if appConfigs.UseOutbound {
		ip, err := GetOutboundIP()
		if err != nil {
			logger.Fatalf("Failed to get outbound address: %s\n ", err.Error())
		}
		host = ip.String()
	} else {
		host = appConfigs.DNSConfigs["host"].(string)
	}

	// Create the dnsmasq config for this team
	teamDomains, err := writeDnsmasqConf(appConfigs.DNSConfigs["team_domain_name"].(string), host, teamNum, dnsmasqConfig)
	if err != nil {
		logger.Fatalf("Failed to generate list of domains for team %d", teamNum)
	}
	host = host + ":53"

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

	// External mask fills in team number from %d
	if appConfigs.DNSConfigs["external_mask"] == nil {
		resp, err := http.Get(mask_proxy_server)
		if err != nil {
			logger.Fatalf("Failed to get external ip address %s\n", err.Error())
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Fatalf("Failed to read response body %s\n", err.Error())
		}
		resp.Body.Close()
		ip_string := string(body)
		external_mask = strings.Join(strings.Split(ip_string, ".")[0:3], ".") + "."
	} else {
		mask := appConfigs.DNSConfigs["external_mask"].(string)
		xs := regexp.MustCompile("x+")
		mask_with_team := xs.ReplaceAll([]byte(mask), []byte(fmt.Sprintf("%02d", teamNum)))
		external_mask = string(mask_with_team)
		if !strings.HasSuffix(external_mask, ".") {
			external_mask = external_mask + "."
		}
	}

	// Create teamDNSServer from external mask which includes team number and the last octect of the dns server of the team
	teamDNSServer := external_mask + appConfigs.DNSConfigs["team_dns_server_last_octet"].(string) + ":53"

	logger.Infof("Team %d: host %s External mask %s internal mask %s dns server %s", teamNum, host, external_mask, internal_mask, teamDNSServer)

	// Use a new server mux instead of a dns.HandleFunction
	serveMux := dns.NewServeMux()
	serveMux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		logger.Infof("Lookup for %s for %s\n", r.Question[0].Name, teamDNSServer)
		responseMsg, err := getResponse(r, internal_mask, external_mask, teamDNSServer, teamDomains)
		if err != nil {
			logger.Errorf("Failed lookup for %s with error: %s\n", r, err.Error())
			responseMsg.SetReply(r)
			w.WriteMsg(responseMsg)
			return
		}

		if len(responseMsg.Answer) > 0 {
			pattern := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
			ipAddress := pattern.FindAllString(responseMsg.Answer[0].String(), -1)

			if len(ipAddress) > 0 {
				logger.Infof("Lookup for %s with ip %s\n", responseMsg.Answer[0].Header().Name, ipAddress[0])
			} else {
				logger.Infof("Lookup for %s with response %s\n", responseMsg.Answer[0].Header().Name, responseMsg.Answer[0])
			}
		}
		responseMsg.SetReply(r)
		w.WriteMsg(responseMsg)
	})

	server := &dns.Server{Addr: host, Net: "udp", Handler: serveMux}
	logger.Infof("Starting at %s\n", host)
	err = server.ListenAndServe()
	if err != nil {
		logger.Fatalf("Failed to start server: %s\n ", err.Error())
	}
}

func writeDnsmasqConf(domainName string, dnsServer string, teamNumber int, dnsmasqConfig string) ([]string, error) {
	//NOTE: this assumes that the tld does not have a period in it
	domainParts := strings.Split(domainName, ".")
	if len(domainParts) != 2 {
		return nil, fmt.Errorf("domain %s not in the expected format", domainName)
	}

	file, err := os.OpenFile(dnsmasqConfig, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open dnsmasq config file %s: %s", dnsmasqConfig, err)
	}
	defer file.Close()

	// Add team# before the domain, in the middle of the domain, and after the domain. Add # in the middle of the domain, too.
	configLines := []string{
		fmt.Sprintf("team%d.%s.%s", teamNumber, domainParts[0], domainParts[1]),
		fmt.Sprintf("%s.team%d.%s", domainParts[0], teamNumber, domainParts[1]),
		fmt.Sprintf("%s.%s.team%d", domainParts[0], domainParts[1], teamNumber),
		fmt.Sprintf("%s%d.%s", domainParts[0], teamNumber, domainParts[1]),
	}

	for _, line := range configLines {
		if _, err := file.WriteString(fmt.Sprintf("server=/%s/%s\n", line, dnsServer)); err != nil {
			log.Fatalf("Failed to write to dnsmasq config file: %s", err)
		}
	}

	// Append the original domain to use for replacement
	configLines = append(configLines, domainName)

	return configLines, nil
}

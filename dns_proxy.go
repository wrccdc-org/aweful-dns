package main

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/miekg/dns"
)

func getResponse(requestMsg *dns.Msg, internal_mask string, external_mask string, defaultServer string, teamDomains []string) (*dns.Msg, error) {
	responseMsg := new(dns.Msg)
	if len(requestMsg.Question) > 0 {
		question := requestMsg.Question[0]

		switch question.Qtype {
		case dns.TypeA:
			answer, err := processTypeA(defaultServer, &question, requestMsg, internal_mask, external_mask, teamDomains)
			if err != nil {
				return responseMsg, err
			}
			responseMsg.Answer = append(responseMsg.Answer, *answer)

		default:
			answer, err := processOtherTypes(defaultServer, &question, requestMsg)
			if err != nil {
				return responseMsg, err
			}
			responseMsg.Answer = append(responseMsg.Answer, *answer)
		}
	}

	return responseMsg, nil
}

func matchesTeamDomain(queryName string, teamDomains []string) (bool, string) {
	for _, domain := range teamDomains {
		if strings.Contains(queryName, domain) {
			// Return true if the matched domain is not the original domain
			return domain != teamDomains[len(teamDomains)-1], domain
		}
	}

	return false, ""
}

func processOtherTypes(dnsServer string, q *dns.Question, requestMsg *dns.Msg) (*dns.RR, error) {
	queryMsg := new(dns.Msg)
	requestMsg.CopyTo(queryMsg)
	queryMsg.Question = []dns.Question{*q}

	msg, err := lookup(dnsServer, queryMsg)
	if err != nil {
		return nil, err
	}

	if len(msg.Answer) > 0 {
		return &msg.Answer[0], nil
	}
	return nil, fmt.Errorf("not found")
}

func processTypeA(dnsServer string, q *dns.Question, requestMsg *dns.Msg, internal_mask string, external_mask string, teamDomains []string) (*dns.RR, error) {
	ip := getIPFromConfigs(q.Name, make(map[string]interface{}))

	if ip == "" {
		queryMsg := new(dns.Msg)
		requestMsg.CopyTo(queryMsg)
		queryMsg.Question = []dns.Question{*q}
		if matches, domain := matchesTeamDomain(queryMsg.Question[0].Name, teamDomains); matches {
			// The last element of teamDomains is the original domain, so replace the matched result with the original in the query
			queryMsg.Question[0].Name = strings.ReplaceAll(queryMsg.Question[0].Name, domain, teamDomains[len(teamDomains)-1])
		}

		msg, err := lookup(dnsServer, queryMsg)
		if err != nil {
			return nil, err
		}

		if len(msg.Answer) > 0 {
			pre_answer := strings.Split(msg.Answer[len(msg.Answer)-1].String(), "\t")
			better_data := pre_answer[len(pre_answer)-1]
			new_addr := strings.Replace(better_data, internal_mask, external_mask, -1)
			answer, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, new_addr))
			if err != nil {
				return nil, err
			}
			return &answer, nil
		}

	} else {
		answer, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
		if err != nil {
			return nil, err
		}
		return &answer, nil
	}
	return nil, fmt.Errorf("not found")
}

func getIPFromConfigs(domain string, configs map[string]interface{}) string {

	for k, v := range configs {
		match, _ := regexp.MatchString(k+"\\.", domain)
		if match {
			return v.(string)
		}
	}
	return ""
}

func GetOutboundIP() (net.IP, error) {

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func lookup(server string, m *dns.Msg) (*dns.Msg, error) {
	dnsClient := new(dns.Client)
	dnsClient.Net = "udp"
	response, _, err := dnsClient.Exchange(m, server)
	if err != nil {
		return nil, err
	}

	return response, nil
}

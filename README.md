# Swiss's Aweful DNS
A fork-ish of [github.com/katakonst/go-dns-proxy](https://github.com/katakonst/go-dns-proxy) designed specifically for WRCCDC's NAT magic.

Goal here is to find an outbound address, and then translate all but the last octet from internal address to external.

# Tirefire's updates
This configures and run multiple DNS proxy servers for different teams. 

The proxy intercepts DNS `A record` requests for names which match one of four patterns for the custom team domains. This strips that portion and based on the number found in the team# it forwards the request on to the configured proxy. It generates dnsmasq configuration files to handle requests with team# in them so that they're then sent on to the proxy listening on a localhost IP address. It then replaces responses in the internal_mask range with the external_mask and forwards that on to the end user.

This relies on JSON configurations located in `/root/aweful-dns-confs`. The config describes an environment with a specific domain:

    `number_of_teams`: The total number of teams. This determines how many instances of DNS servers will be started.
    `internal_mask` and `external_mask`: Used to generate IP addresses for internal and external DNS queries. The application replaces 'xx' in the external mask with the team number.
    `team_dns_server_last_octet`: The last octet used to form the complete IP address of each team's DNS server.
    `team_domain_name`: The base domain name used for DNS queries.

Multiple json files can exist in /root/aweful-dns-confs and it will handle each different environment.

An init script is in `./etc/init.d/aweful-dns` which places logs in `/var/log/aweful-dns.log`.

## Example
```json
{
    "number_of_teams": 30,
    "internal_mask": "192.168.220.",
    "external_mask": "10.100.1xx.",
    "team_dns_server_last_octet": "132",
    "team_domain_name": "example.com"
}
```

This will create 30 proxies listening on a localhost address, which will strip team# from the request and forward it on to the external mask + last octet of the dns server.
So if a request for team6.example.com comes in, dnsmasq will have a registered server for that, it will then get sent to the localhost ip address that handles team 6, the proxy will receive the request, strip the team6 from the request, then forward it on to 10.100.106.132. When the response is recieved that includes the 192.168.220.12 it modifies that to be 10.100.106.12 and returns that to the original requester.

## Generation of dnsmasq Configurations

The application automatically generates dnsmasq configuration entries for each team. It creates four entries per team, which follow these patterns:

    team[teamnumber].domain.tld
    domain.team[teamnumber].tld
    domain.tld.team[teamnumber]
    domain[teamnumber].tld

The configurations are appended to the dnsmasq configuration file specified in the application's startup arguments.

## Assumptions

  * dnsmasq Installation: The application assumes that dnsmasq is already installed and configured to read from /etc/dnsmasq.d/aweful-dns.conf on the host system.
  * Team Name Positioning: The team number is dynamically inserted into the domain name in four different positions for versatility in DNS configurations.
  * Configuration Directory: The application expects /root/aweful-dns-confs directory for configuration files, or as passed in the startup arguments.
  * Network: It assumes all networks are /24 and that there won't be more than 99 teams (101-199), and that all teams will have the same DNS server IP address, and that there will be a 1-to-1 NAT from external to internal.

# DNS Proxy
A modification of the simple DNS proxy written in go based on [github.com/miekg/dns](https://github.com/miekg/dns)

# go-dns
A DNS resolver written in Go. Inspired by John Crickett's coding [challenges](https://codingchallenges.fyi/challenges/challenge-dns-resolver).

```
Usage: ./dns example.com
```

For instance, to find the IP address of [campusninja.net](campusninja.net):

```
$ ./dns campusninja.net

Querying 192.203.230.10 for campusninja.net
Querying 192.35.51.30 for campusninja.net
Querying 192.43.172.30 for campusninja.net
Querying 192.31.80.30 for campusninja.net
Querying 192.5.6.30 for campusninja.net
Querying 192.33.14.30 for campusninja.net
Querying 192.12.94.30 for campusninja.net
Querying 192.203.230.10 for max.ns.cloudflare.com
Querying 192.35.51.30 for max.ns.cloudflare.com
Querying 162.159.4.8 for max.ns.cloudflare.com
Querying 173.245.59.132 for campusninja.net
188.114.96.9
```

We get the IPv4 address: `188.114.96.9`.
Let's check with `dig`:

```
$ dig campusninja.net

; <<>> DiG 9.18.12-0ubuntu0.22.04.3-Ubuntu <<>> campusninja.net
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 42936
;; flags: qr rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 65494
;; QUESTION SECTION:
;campusninja.net.               IN      A

;; ANSWER SECTION:
campusninja.net.        300     IN      A       188.114.96.9
campusninja.net.        300     IN      A       188.114.97.9
```

As we'd hope, the first answer matches!

I chose to display slightly different information. My resolver shows every query it has made to another server while resolving the original query.
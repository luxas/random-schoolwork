# ping

A PoC reimplementation of the famous `ping` command. Supports displaying RTT times, computing statistics on those,
specifying a send interval, listen address, and max RTT time. Uses raw sockets, hence `sudo` is used in the examples.
Only IPv4 is supported. Inspired by: [github.com/sparrc/go-ping](https://github.com/sparrc/go-ping).

## Building

```bash
make
```

## Usage

Just like normal ping. Some examples:

```console
$ sudo bin/ping 1.1.1.1
PING 1.1.1.1 (1.1.1.1): 16 data bytes
64 bytes from 1.1.1.1: icmp_seq=0 ttl=0 time=10.758963ms
64 bytes from 1.1.1.1: icmp_seq=1 ttl=0 time=14.456447ms
64 bytes from 1.1.1.1: icmp_seq=2 ttl=0 time=10.548685ms
64 bytes from 1.1.1.1: icmp_seq=3 ttl=0 time=10.213742ms
64 bytes from 1.1.1.1: icmp_seq=4 ttl=0 time=11.34282ms
^C
--- 1.1.1.1 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 4460 ms
rtt min/avg/max/sdev = 10.214/11.464/14.456/1.722 ms
```

Support for resolving DNS A records:

```console
$ sudo bin/ping www.google.com
PING www.google.com (216.58.207.228): 16 data bytes
64 bytes from 216.58.207.228: icmp_seq=0 ttl=0 time=15.929477ms
64 bytes from 216.58.207.228: icmp_seq=1 ttl=0 time=17.670992ms
64 bytes from 216.58.207.228: icmp_seq=2 ttl=0 time=16.468073ms
64 bytes from 216.58.207.228: icmp_seq=3 ttl=0 time=23.381702ms
^C
--- www.google.com ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3676 ms
rtt min/avg/max/sdev = 15.929/18.363/23.382/3.424 ms
```

Support for setting a maximum TTL (for traceroute-like functionality):

```console
sudo bin/ping --ttl 2 1.1.1.1
PING 1.1.1.1 (1.1.1.1): 16 data bytes
Error when receiving: From 85.134.88.1 icmp_seq=0 Time To Live exceeded
Error when receiving: From 85.134.88.1 icmp_seq=1 Time To Live exceeded
Error when receiving: From 85.134.88.1 icmp_seq=2 Time To Live exceeded
Error when receiving: From 85.134.88.1 icmp_seq=3 Time To Live exceeded
Error when receiving: From 85.134.88.1 icmp_seq=4 Time To Live exceeded
^C
--- 1.1.1.1 ping statistics ---
5 packets transmitted, 0 received, 100% packet loss, time 4358 ms
rtt min/avg/max/sdev = 0.000/0.000/0.000/0.000 ms
```

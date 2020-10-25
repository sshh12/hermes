# hermes

> A simple TCP forwarder & Ngrok alternative.

### Features

||Hermes|Ngrok (Pro) |
|--|--|--|
|Protocol Support| SSH, HTTP(S), etc (anything TCP) | SSH, HTTP(S), etc (anything TCP)|
| Pricing| ~$5/mo dependent on VPS | $8.25/mo |
|Reserved TCP Addresses| inf* | 2 |
|Max Connections/minute| inf* | 60|
|Max Tunnels/Process| inf* | 12|
|Max Online Processes| inf* | 2|
|Pick Arbitrary Remote Port| yes| no|
|Custom Domains| yes| yes (<=5)|
|TLS| yes| yes|

*No software restriction, although things will start to break down at some point

### Basic Usage

#### Server Setup

Setup a basic server on a linux VPS (e.g. DigitalOcean droplet):

```
$ wget https://github.com/sshh12/hermes/releases/download/$VERSION/client-$VERSION-linux-amd64.tar.gz
$ tar -xzf client-$VERSION-linux-amd64.tar.gz
$ ./server
```

Change `$VERSION` to the latest [release name](https://github.com/sshh12/hermes/releases).

#### Client Setup

Run a simple web server:

```
$ python -m http.server 8080
```

Download the latest client [release](https://github.com/sshh12/hermes/releases), then:

```
$ ./hermes -server $SERVER_IP -save
$ ./hermes 8080 8000
```

This will forward all connections from `$SERVER_IP:8000` to `localhost:8080` without needing to port forward on the client network.

or use this syntax for multiple tunnels:

```
$ ./hermes 5000 5001 3000 3001
```

This will forward `$SERVER_IP:5001` to `localhost:5000` and `$SERVER_IP:3001` to `localhost:3000`.

### TLS Usage

#### Server Setup

```
$ openssl ecparam -genkey -name secp384r1 -out server.key
$ openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
$ ./server
```

#### Client Setup

```
$ ./hermes -use_tls -tls_skip_verify -save
$ ./hermes 8080 8000
```
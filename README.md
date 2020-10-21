# hermes

> A simple TCP forwarder & Ngrok alternative.

### Usage

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
$ client -port LOCAL_PORT -rport REMOTE_PORT -hhost SERVER_IP
for example:
$ client -port 8080 -rport 8000 -hhost 161.12.12.123
```

This will forward all connections to `161.12.12.123:8000` to `localhost:8080` without needing to port forward on the client network.
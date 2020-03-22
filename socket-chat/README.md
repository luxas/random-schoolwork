# socket-chat

A small PoC implementation of a chat application using a TCP connection over an Unix socket.
It features a server component and a client. Clients are registered by name, and may either
message each other directly or through group chats they create and add each other to.

## Building

```bash
make
```

## Usage

You need three different terminal windows:

```bash
rm -f /tmp/server.sock && bin/server
```

```bash
bin/client --name foo
```

```bash
bin/client --name bar
```

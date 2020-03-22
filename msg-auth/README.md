# msg-auth

A PoC implementation of sending messages over some kind of channel, where you want to be able to verify receiver-side that
a) the sender is legitimate (as they share the same shared secret as me) and b) the message has not been tampered with
during transmission. This example uses SHA-3-512 as the default hashing mechanism to verify this, but many other
algorithms can be chosen using a flag.

## Building

```bash
make
```

## Usage

You may start two independent instances of the program, give it a secret, and `hash` a message as follows:

```console
$ bin/msg-auth --secret my-secret
> Usage:
> verify,message-on-the-wire -- Verify if the message received may be trusted
> hash,message -- Hash the message that should be transferred to the receiver
> help -- Show this help message
> quit -- Quit the program
$ hash,Hello out there!
> Message to send:
> 10Hello out there!33ab81e36485f6c20d20b325ffca9f845e42cb65b3e01a112e4f27feed4da0ada5af2521e5e0c7e5222f42a1b7560f59dafec8a9268715de14b1429ea3beade2
```

Then, on the "receiver-side", you can verify the message. When giving it the exact string as got by the `hash` output
above, it works, but changing even one char in the end (e.g. the ending `2` to a `1`), is detected and results in an
error.

```console
$ bin/msg-auth --secret my-secret
> Usage:
> hash,message -- Hash the message that should be transferred to the receiver
> verify,message-on-the-wire -- Verify if the message received may be trusted
> help -- Show this help message
> quit -- Quit the program
$ verify,10Hello out there!33ab81e36485f6c20d20b325ffca9f845e42cb65b3e01a112e4f27feed4da0ada5af2521e5e0c7e5222f42a1b7560f59dafec8a9268715de14b1429ea3beade2
> Message verified! You can trust this message
$ verify,10Hello out there!33ab81e36485f6c20d20b325ffca9f845e42cb65b3e01a112e4f27feed4da0ada5af2521e5e0c7e5222f42a1b7560f59dafec8a9268715de14b1429ea3beade1
> Message has been tampered with! Don't trust this message!!
```

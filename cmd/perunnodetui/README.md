# Perun Node TUI Client [![join the chat][rocketchat-image]][rocketchat-url]

[rocketchat-url]: https://chat.hyperledger.org/channel/perun
[rocketchat-image]: https://open.rocket.chat/images/join-chat.svg

Perun Node TUI is a text based UI client for trying out the perun payment
channels in a graphical way. This software is completely independent from the
perun-node. It uses the grpc interface to interact with it.

To use this application, we need to start the `perun node` and a blockchain
node (preferably using `ganache-cli`). Detailed steps for setting up these
components are described in the
[project documentation](https://labs.hyperledger.org/perun-doc/node/user_guide.html#getting-started).

Once you have started the perun-node and the ganache-cli nodes, you can start
the `perunnodetui` by running the below commands in two different terminals
(from the perun-node directory).


```
cd cmd/perunnodetui
go build && ./perunnodetui -alice -deploy
```

```
cd cmd/perunnodetui
go build && ./perunnodetui -bob
```

The `alice` and `bob` flags in the above command will load the default
configuration values for the respective users into the fields in the `connect
screen`. The `deploy` flag will deploy the perun contracts to ganache-cli node.
The contracts will be deployed only once and will be used by all participants
in the perun network.

Once in the `connect screen`, press `connect` button to connect with the perun
node. After connecting with the node, the application will switch to `dashboard
screen`.

![Dashboard image not found](dashboard.png?raw=true "Dashboard screen")

The `dashboard screen` shows,

- Address and balance of the user's on-chain account in the top left.
- Time in the top right corner. This will be used as reference to see if
  notifications have expired.
- List of all channels in a table format in the middle of the screen.
- Command Box and Log Box in the bottom of the screen.

To interact with the application, type commands in the command box. Supported
commands are show as place holder text within the box. Here is a brief
tutorial.

1. To open a channel:

```
open <peer-name> <own-balance> <peer-balance> 
```

Once the command is entered, the channel shows up in the table in `Open` phase
for sender and a request is sent to peer.

2. To accept or reject a channel:

The channel request will show up in the peer's table in `Open` phase along with
a `timeout` before which the request should be responded to. Use `acc` or
`rej` followed by the `S.No` of the channel to send the response.

```
acc <S.No>
```

If peer accepts the channel before timeout expires, it will be funded on the
blockchain from the users' accounts. The decrease in balances of both the users
can be seen in the top left boxes in their respective screens.

Once it is funded, the phase of the channel will change to `Transact`.

3. To do off-chain transactions:

Once it is in `Transact` phase, use `send` and `req` commands to send and
request funds respectively.

```
send <S.No> <amount>
```

The updated balance will show up in the updates section of that channel in the
table with the status as `For Peer`. It will also show up on the peer's screen
with the status as `For User`.

Use `acc` and `rej` commands in the same way as in step 2 for responding to the
update. If accepted, the current balance of the channel will be updated. If
rejected, the current will remain unchanged. One can do any number of off-chain
transactions this way.

4. Close the channel:

To withdraw funds as per the latest balance of the channel, use the `close`
command.

```
close <S.No>
```

Once `close` command is entered, the channel phase will be marked as `Register`
and a special finalizing update will be sent to the peer. In this update, the
balances will remain unchanged, but it will be marked as final. This will be
shown as ` F` appended after the version number.

Peer can accept or reject this update. If accepted, the new final state will be
registered on the blockchain.  If rejected or if timeout expires, the last
known state will be registered on the blockchain. The difference is that, when
accepted, the final state serves as a proof that both parties have agreed to
conclude a particular state on the blockchain. Hence the state will be
immediately settled on the blockchain and users can withdraw their funds
without any delay.

On the other hand, if a non-final state was registered, both parties will have
to wait for a specific period of time to withdraw their balances. This waiting
period provides an opportunity for the other participant to refute with the
latest state if an older state was registered. The aspects of refuting (if
required), waiting out and withdrawing the funds will be automatically taken
care of by the node.

Once the channel is settled, phase will be marked as `Settle` on both the
dashboards and balance withdrawal will be initiated by the node. Once the
balances are withdrawn, it will change to `Closed`.

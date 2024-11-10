# FlitVPN

FlitVPN is a simple, low-cost solution to extend your [Tailscale](https://tailscale.com/) network to operate as an international VPN.

By temporarily deploying Tailscale exit nodes on cloud infrastructure, FlitVPN allows you to route traffic through regions outside your local network. This setup is ideal for routing traffic through, and accessing content from, different geographic locations—without the commitment of a third-party VPN subscription.

For now, FlitVPN only supports deploying nodes to DigitalOcean.


## Usage

Routing traffic is as simple as running

```
flit up <region>
```

and selecting the newly created exit node in your Tailscale client.

Once you're done, deselect the exit node in your Tailscale client and run

```
flit down
```

to remove the Tailscale node and avoid ongoing charges from your cloud provider. For many cases of light personal use, your usage may even fall within your provider's free tier limit, costing you nothing at all!

To get started, run

```
flit login
```

and enter a [DigitalOcean personal access token](https://docs.digitalocean.com/reference/api/create-personal-access-token/) and a [Tailscale auth key](https://tailscale.com/kb/1085/auth-keys) and you're ready to go!


## Possible Future Features

 - Other cloud providers for node deployment
 - Environment variables for authentication tokens, enabling use in scripting environments
 - Support for more than one exit node

## A note on third party VPNs

High quality VPN providers absolutely have their advantages, and FlitVPN isn't necessarily designed as a replacement for these. However if you're already a Tailscale user and are looking for a simple, low cost way to occasionally route your traffic around the world, FlitVPN might be worth a try!

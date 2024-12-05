# FlitVPN

FlitVPN is a simple, low-cost solution to extend your [Tailscale](https://tailscale.com/) network to operate as an international VPN.

By temporarily deploying Tailscale [exit nodes](https://tailscale.com/kb/1103/exit-nodes) on cloud infrastructure, FlitVPN allows you to route traffic through regions outside your local network. This setup is ideal for routing traffic through, and accessing content from, different geographic locations—without the commitment of a third-party VPN subscription.

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


## Completions

To load completions:

### Bash

```bash
source < (flit completion bash)
```

To load completions for each session, execute once:

```bash
# Linux
flit completion bash > /etc/bash_completion.d/flit-vpn

# macOS
flit completion bash > $(brew --prefix)/etc/bash_completion.d/flit-vpn
```

### Zsh

To load completions for each session, execute once:

```bash
flit completion zsh > "${fpath[1]}/_%[1]s"
```

### Fish

```fish
flit completion fish | source
```

To load completions for each session, execute once:

```fish
flit completion fish > ~/.config/fish/completions/flit-vpn.fish
```

### PowerShell

```powershell
PS> flit completion powershell | Out-String | Invoke-Expression
```

To load completions for every new session, run:

```
PS> flit completion powershell > flit-vpn.ps1
```

and source this file from your PowerShell profile.


## Possible Future Features

 - Other cloud providers for node deployment
 - Environment variables for authentication tokens, enabling use in scripting environments
 - Support for more than one exit node

## A note on third party VPNs

High quality VPN providers absolutely have their advantages, and FlitVPN isn't necessarily designed as a replacement for these. However if you're already a Tailscale user and are looking for a simple, low cost way to occasionally route your traffic around the world, FlitVPN might be worth a try!

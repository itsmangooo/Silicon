package wireguard

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	networkprovider "github.com/itsmangooo/Silicon/backend/internal/providers/network"
)

type Provider struct{}

func (Provider) EnsureIdentity(ctx context.Context, executor connection.CommandExecutor, interfaceName string) (string, error) {
	if err := validInterface(interfaceName); err != nil {
		return "", err
	}
	path, cleanup, err := executor.WriteFile(ctx, "silicon-wireguard-identity", []byte(identityScript), 0700)
	if err != nil {
		return "", fmt.Errorf("stage WireGuard identity operation: %w", err)
	}
	defer cleanup(context.Background())
	output, err := executor.Output(ctx, nil, "sh", path, interfaceName)
	if err != nil {
		return "", fmt.Errorf("prepare host-side WireGuard identity: %w", err)
	}
	publicKey := strings.TrimSpace(output)
	if !validPublicKey(publicKey) {
		return "", errors.New("WireGuard returned an invalid public key")
	}
	return publicKey, nil
}

func (Provider) Apply(ctx context.Context, executor connection.CommandExecutor, configuration networkprovider.NodeConfiguration) (string, error) {
	if err := validate(configuration); err != nil {
		return "", err
	}
	wgConfig := renderWireGuard(configuration)
	nftConfig := renderFirewall(configuration)
	corefile := renderCoreDNS(configuration)
	configurationHash := sha256.Sum256([]byte(wgConfig + "\n" + nftConfig + "\n" + corefile))
	configPath, cleanConfig, err := executor.WriteFile(ctx, "silicon-wireguard-config", []byte(wgConfig), 0600)
	if err != nil {
		return "", fmt.Errorf("stage WireGuard configuration: %w", err)
	}
	defer cleanConfig(context.Background())
	nftPath, cleanNFT, err := executor.WriteFile(ctx, "silicon-network-firewall", []byte(nftConfig), 0600)
	if err != nil {
		return "", fmt.Errorf("stage network policy: %w", err)
	}
	defer cleanNFT(context.Background())
	dnsPath, cleanDNS, err := executor.WriteFile(ctx, "silicon-network-dns", []byte(corefile), 0600)
	if err != nil {
		return "", fmt.Errorf("stage service discovery: %w", err)
	}
	defer cleanDNS(context.Background())
	scriptPath, cleanScript, err := executor.WriteFile(ctx, "silicon-wireguard-apply", []byte(applyScript), 0700)
	if err != nil {
		return "", fmt.Errorf("stage network apply operation: %w", err)
	}
	defer cleanScript(context.Background())
	if _, err = executor.Output(ctx, nil, "sh", scriptPath, configuration.InterfaceName, configPath, nftPath, dnsPath, firewallTable(configuration.NetworkID), configuration.OrganizationID, configuration.NetworkID, boolArg(configuration.IsHub)); err != nil {
		return "", fmt.Errorf("apply WireGuard network: %w", err)
	}
	return hex.EncodeToString(configurationHash[:]), nil
}

func (Provider) Remove(ctx context.Context, executor connection.CommandExecutor, interfaceName, networkID string) error {
	if err := validInterface(interfaceName); err != nil {
		return err
	}
	path, cleanup, err := executor.WriteFile(ctx, "silicon-wireguard-remove", []byte(removeScript), 0700)
	if err != nil {
		return fmt.Errorf("stage network removal operation: %w", err)
	}
	defer cleanup(context.Background())
	if _, err = executor.Output(ctx, nil, "sh", path, interfaceName, firewallTable(networkID), networkID); err != nil {
		return fmt.Errorf("remove WireGuard network: %w", err)
	}
	return nil
}

func renderWireGuard(configuration networkprovider.NodeConfiguration) string {
	var value strings.Builder
	value.WriteString("[Interface]\nAddress = " + configuration.Address + "/32\nPrivateKey = __SILICON_HOST_KEY__\n")
	if configuration.IsHub {
		value.WriteString("ListenPort = " + strconv.Itoa(configuration.ListenPort) + "\n")
	}
	peers := append([]networkprovider.Peer(nil), configuration.Peers...)
	sort.Slice(peers, func(i, j int) bool { return peers[i].PublicKey < peers[j].PublicKey })
	for _, peer := range peers {
		value.WriteString("\n[Peer]\nPublicKey = " + peer.PublicKey + "\nAllowedIPs = " + strings.Join(peer.AllowedCIDRs, ", ") + "\n")
		if peer.Endpoint != "" {
			value.WriteString("Endpoint = " + peer.Endpoint + "\n")
		}
		if peer.PersistentKeepalive > 0 {
			value.WriteString("PersistentKeepalive = " + strconv.Itoa(peer.PersistentKeepalive) + "\n")
		}
	}
	return value.String()
}

func renderCoreDNS(configuration networkprovider.NodeConfiguration) string {
	if !configuration.IsHub {
		return "# DNS is hosted by the WireGuard hub.\n"
	}
	services := append([]networkprovider.Service(nil), configuration.Services...)
	sort.Slice(services, func(i, j int) bool { return services[i].Hostname < services[j].Hostname })
	var value strings.Builder
	value.WriteString("internal:53 {\n    bind " + configuration.DNSServer + "\n    errors\n    hosts {\n")
	for _, service := range services {
		value.WriteString("        " + service.Address + " " + service.Hostname + "\n")
	}
	value.WriteString("        fallthrough\n    }\n}\n.:53 {\n    bind " + configuration.DNSServer + "\n    errors\n    cache 30\n    forward . /etc/resolv.conf\n}\n")
	return value.String()
}

func renderFirewall(configuration networkprovider.NodeConfiguration) string {
	table := firewallTable(configuration.NetworkID)
	var value strings.Builder
	value.WriteString("delete table inet " + table + "\ntable inet " + table + " {\n  chain overlay_ingress {\n    type filter hook prerouting priority -110; policy accept;\n")
	value.WriteString("    iifname \"" + configuration.InterfaceName + "\" ip daddr " + configuration.DNSServer + " udp dport 53 accept\n")
	rules := append([]networkprovider.AccessRule(nil), configuration.Rules...)
	sort.Slice(rules, func(i, j int) bool { return ruleKey(rules[i]) < ruleKey(rules[j]) })
	for _, rule := range rules {
		for _, source := range rule.SourceAddresses {
			value.WriteString("    iifname \"" + configuration.InterfaceName + "\" ip saddr " + source + " ip daddr " + rule.Destination.Address + " " + rule.Destination.Protocol + " dport " + strconv.Itoa(rule.Destination.Port) + " " + rule.Action + "\n")
		}
	}
	for _, service := range configuration.Services {
		if service.Address == configuration.Address {
			value.WriteString("    iifname \"" + configuration.InterfaceName + "\" ip daddr " + service.Address + " " + service.Protocol + " dport " + strconv.Itoa(service.Port) + " drop\n")
		}
	}
	value.WriteString("  }\n}\n")
	return value.String()
}

func validate(configuration networkprovider.NodeConfiguration) error {
	if err := validInterface(configuration.InterfaceName); err != nil {
		return err
	}
	if _, err := netip.ParseAddr(configuration.Address); err != nil {
		return errors.New("invalid node address")
	}
	if _, err := netip.ParsePrefix(configuration.CIDR); err != nil {
		return errors.New("invalid network CIDR")
	}
	if configuration.ListenPort < 1 || configuration.ListenPort > 65535 {
		return errors.New("invalid WireGuard listen port")
	}
	for _, peer := range configuration.Peers {
		if !validPublicKey(peer.PublicKey) {
			return errors.New("peer public key is invalid")
		}
		for _, cidr := range peer.AllowedCIDRs {
			if _, err := netip.ParsePrefix(cidr); err != nil {
				return errors.New("peer allowed CIDR is invalid")
			}
		}
	}
	for _, service := range configuration.Services {
		if _, err := netip.ParseAddr(service.Address); err != nil || service.Port < 1 || service.Port > 65535 || (service.Protocol != "tcp" && service.Protocol != "udp") {
			return errors.New("network service is invalid")
		}
	}
	return nil
}

func validPublicKey(value string) bool {
	if len(value) != 44 || strings.ContainsAny(value, "\r\n \t") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func validInterface(value string) error {
	if len(value) < 3 || len(value) > 15 || !strings.HasPrefix(value, "si") {
		return errors.New("invalid Silicon WireGuard interface")
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
			return errors.New("invalid Silicon WireGuard interface")
		}
	}
	return nil
}
func firewallTable(networkID string) string {
	value := strings.ReplaceAll(networkID, "-", "")
	if len(value) > 12 {
		value = value[:12]
	}
	return "silicon_" + value
}
func ruleKey(rule networkprovider.AccessRule) string {
	priority := "1"
	if rule.Action == "deny" {
		priority = "0"
	}
	return rule.Destination.Hostname + "/" + rule.Destination.Protocol + "/" + strconv.Itoa(rule.Destination.Port) + "/" + priority + "/" + strings.Join(rule.SourceAddresses, ",")
}
func boolArg(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

const identityScript = `set -eu
iface="$1"
case "$iface" in si[0-9a-z]*) ;; *) echo "invalid interface" >&2; exit 2;; esac
if [ "$(id -u)" -eq 0 ]; then SUDO=""; elif command -v sudo >/dev/null 2>&1; then SUDO="sudo -n"; else echo "root or passwordless sudo is required" >&2; exit 1; fi
command -v wg >/dev/null 2>&1 || { echo "wireguard-tools is required" >&2; exit 1; }
$SUDO install -d -m 0700 /var/lib/silicon/wireguard
key="/var/lib/silicon/wireguard/${iface}.key"
if ! $SUDO test -s "$key"; then
  umask 077
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT HUP INT TERM
  wg genkey > "$tmp"
  $SUDO install -m 0600 "$tmp" "$key"
  rm -f "$tmp"
  trap - EXIT HUP INT TERM
fi
$SUDO cat "$key" | wg pubkey
`

const applyScript = `set -eu
iface="$1"; template="$2"; nftfile="$3"; corefile="$4"; table="$5"; org="$6"; network="$7"; hub="$8"
case "$iface" in si[0-9a-z]*) ;; *) echo "invalid interface" >&2; exit 2;; esac
case "$table" in silicon_[0-9a-z]*) ;; *) echo "invalid firewall table" >&2; exit 2;; esac
if [ "$(id -u)" -eq 0 ]; then SUDO=""; elif command -v sudo >/dev/null 2>&1; then SUDO="sudo -n"; else echo "root or passwordless sudo is required" >&2; exit 1; fi
for tool in wg wg-quick nft docker; do command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required" >&2; exit 1; }; done
key="/var/lib/silicon/wireguard/${iface}.key"
$SUDO test -s "$key" || { echo "WireGuard host identity is missing" >&2; exit 1; }
$SUDO install -d -m 0700 /var/lib/silicon/wireguard /var/lib/silicon/wireguard/candidate /etc/wireguard
if [ "$hub" = "true" ]; then
  forward_tmp="$(mktemp)"
  printf 'net.ipv4.ip_forward=1\n' > "$forward_tmp"
  $SUDO install -m 0644 "$forward_tmp" "/etc/sysctl.d/90-silicon-${iface}.conf"
  rm -f "$forward_tmp"
  $SUDO sysctl -w net.ipv4.ip_forward=1 >/dev/null
fi
candidate="/var/lib/silicon/wireguard/candidate/${iface}.conf"
backup="/var/lib/silicon/wireguard/${iface}.previous.conf"
{
  while IFS= read -r line; do
    if [ "$line" = "PrivateKey = __SILICON_HOST_KEY__" ]; then printf 'PrivateKey = '; $SUDO cat "$key"; else printf '%s\n' "$line"; fi
  done < "$template"
} | $SUDO tee "$candidate" >/dev/null
$SUDO chmod 0600 "$candidate"
$SUDO wg-quick strip "$candidate" >/dev/null
if $SUDO test -f "/etc/wireguard/${iface}.conf"; then $SUDO cp "/etc/wireguard/${iface}.conf" "$backup"; fi
$SUDO install -m 0600 "$candidate" "/etc/wireguard/${iface}.conf"
$SUDO wg-quick down "$iface" >/dev/null 2>&1 || true
if ! $SUDO wg-quick up "$iface"; then
  if $SUDO test -f "$backup"; then $SUDO install -m 0600 "$backup" "/etc/wireguard/${iface}.conf"; $SUDO wg-quick up "$iface" || true; fi
  echo "WireGuard configuration failed; previous configuration restored" >&2; exit 1
fi
$SUDO nft list table inet "$table" >/dev/null 2>&1 || $SUDO nft add table inet "$table"
$SUDO nft -c -f "$nftfile"
$SUDO nft -f "$nftfile"
dnsdir="/var/lib/silicon/network-dns/${network}"
$SUDO install -d -m 0755 "$dnsdir"
$SUDO install -m 0644 "$corefile" "$dnsdir/Corefile"
name="silicon-coredns-${network}"
if [ "$hub" = "true" ]; then
  docker pull coredns/coredns:1.12.4 >/dev/null
  docker rm -f "${name}-previous" >/dev/null 2>&1 || true
  if docker inspect "$name" >/dev/null 2>&1; then docker stop "$name" >/dev/null; docker rename "$name" "${name}-previous"; fi
  if ! docker create --name "$name" --restart unless-stopped --network host --read-only --cap-drop ALL --cap-add NET_BIND_SERVICE --security-opt no-new-privileges --label silicon.managed=true --label "silicon.organization_id=$org" --label "silicon.network_id=$network" --mount "type=bind,source=$dnsdir/Corefile,target=/Corefile,readonly" -- coredns/coredns:1.12.4 -conf /Corefile >/dev/null || ! docker start "$name" >/dev/null; then
    docker rm -f "$name" >/dev/null 2>&1 || true
    if docker inspect "${name}-previous" >/dev/null 2>&1; then docker rename "${name}-previous" "$name"; docker start "$name" >/dev/null || true; fi
    echo "CoreDNS failed; previous DNS container restored" >&2; exit 1
  fi
  docker rm -f "${name}-previous" >/dev/null 2>&1 || true
fi
`

const removeScript = `set -eu
iface="$1"; table="$2"; network="$3"
case "$iface" in si[0-9a-z]*) ;; *) echo "invalid interface" >&2; exit 2;; esac
case "$table" in silicon_[0-9a-z]*) ;; *) echo "invalid firewall table" >&2; exit 2;; esac
if [ "$(id -u)" -eq 0 ]; then SUDO=""; elif command -v sudo >/dev/null 2>&1; then SUDO="sudo -n"; else echo "root or passwordless sudo is required" >&2; exit 1; fi
$SUDO wg-quick down "$iface" >/dev/null 2>&1 || true
$SUDO rm -f -- "/etc/wireguard/${iface}.conf" "/var/lib/silicon/wireguard/${iface}.key" "/var/lib/silicon/wireguard/candidate/${iface}.conf" "/var/lib/silicon/wireguard/${iface}.previous.conf"
$SUDO rm -f -- "/etc/sysctl.d/90-silicon-${iface}.conf"
$SUDO nft delete table inet "$table" >/dev/null 2>&1 || true
name="silicon-coredns-${network}"
docker rm -f "$name" "${name}-previous" >/dev/null 2>&1 || true
`

var _ networkprovider.Provider = Provider{}

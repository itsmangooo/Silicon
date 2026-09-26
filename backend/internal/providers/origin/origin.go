package origin

import "github.com/google/uuid"

type Target struct {
	ApplicationID   uuid.UUID `json:"applicationId"`
	ServerID        uuid.UUID `json:"serverId"`
	ConnectionType  string    `json:"connectionType"`
	Address         string    `json:"address"`
	Port            int       `json:"port"`
	Protocol        string    `json:"protocol"`
	DNSRecordType   string    `json:"dnsRecordType"`
	PublicDNS       bool      `json:"publicDns"`
	Tunnel          bool      `json:"tunnel"`
	TunnelPreferred bool      `json:"tunnelPreferred"`
}

func (t Target) ServiceURL() string {
	return t.Protocol + "://127.0.0.1:" + itoa(t.Port)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}

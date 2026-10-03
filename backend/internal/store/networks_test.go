package store

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
)

func TestNextNetworkAddressIsDeterministicAndSkipsUsed(t *testing.T) {
	prefix := netip.MustParsePrefix("10.44.0.0/29")
	first, err := nextAddress(prefix, nil)
	if err != nil || first != "10.44.0.1" {
		t.Fatalf("first=%q err=%v", first, err)
	}
	next, err := nextAddress(prefix, []netip.Addr{netip.MustParseAddr("10.44.0.1"), netip.MustParseAddr("10.44.0.2")})
	if err != nil || next != "10.44.0.3" {
		t.Fatalf("next=%q err=%v", next, err)
	}
}

func TestNetworkAddressPoolExhaustion(t *testing.T) {
	prefix := netip.MustParsePrefix("10.44.0.0/30")
	_, err := nextAddress(prefix, []netip.Addr{netip.MustParseAddr("10.44.0.1"), netip.MustParseAddr("10.44.0.2")})
	if err == nil {
		t.Fatal("exhausted pool returned an address")
	}
}

func TestNetworkInterfaceNameIsStableAndLinuxSafe(t *testing.T) {
	id := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	if got := NetworkInterfaceName(id); got != "si0123456789ab" || len(got) > 15 {
		t.Fatalf("interface=%q", got)
	}
}

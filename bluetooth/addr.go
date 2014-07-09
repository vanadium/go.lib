package bluetooth

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Network string for net.Addr implementations used by the bluetooth package.
const Network = "bluetooth"

// addr represents an RFCOMM over bluetooth address in the <MAC-channelID>
// format, where channelID denotes one of the available RFCOMM channels.
//
// It implements the net.Addr interface.
type addr struct {
	mac     net.HardwareAddr
	channel int
}

// anyMAC is a MAC address "00:00:00:00:00:00", which means first available
// (bluetooth) device.
var anyMAC net.HardwareAddr

func init() {
	var err error
	if anyMAC, err = net.ParseMAC("00:00:00:00:00:00"); err != nil {
		panic("can't parse address 00:00:00:00:00:00")
	}
}

// parseAddress parses an address string in the <MAC-channelID> format (e.g.,
// "01:23:45:67:89:AB-1").  It returns an error if the address is in the wrong
// format.  It is legal for a MAC address sub-part to be empty, in which case
// it will be treated as anyMAC (i.e., "00:00:00:00:00:00").
func parseAddress(address string) (*addr, error) {
	parts := strings.Split(address, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("too many or too few \"-\" in address: %s", address)
	}
	ms := parts[0]
	ps := parts[1]
	if len(ms) == 0 {
		channel, err := strconv.ParseInt(ps, 0, 32)
		if err != nil {
			return nil, err
		}
		return &addr{anyMAC, int(channel)}, nil
	} else {
		mac, err := net.ParseMAC(ms)
		if err != nil {
			return nil, err
		}
		channel, err := strconv.ParseInt(ps, 0, 32)
		if err != nil {
			return nil, err
		}
		return &addr{mac, int(channel)}, nil
	}
}

// Implements the net.Addr interface.
func (a *addr) Network() string {
	return Network
}

// Implements the net.Addr interface.
func (a *addr) String() string {
	return fmt.Sprintf("%s-%d", a.mac, a.channel)
}

// isAnyMAC returns true iff the mac address is "any" (i.e.,
// "00:00:00:00:00:00")
func (a *addr) isAnyMAC() bool {
	return bytes.Equal(a.mac, anyMAC)
}

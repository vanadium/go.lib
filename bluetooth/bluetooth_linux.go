// +build veyronbluetooth,!android

package bluetooth

import (
	"fmt"
	"math"
	"net"
	"syscall"
	"time"
	"unsafe"

	"veyron/lib/unit"
	"veyron2/vlog"
)

// // Explicitly link libbluetooth and other libraries as "go build" cannot
// // figure out these dependencies..
// #cgo LDFLAGS: -lbluetooth
// #include <bluetooth/bluetooth.h>
// #include <bluetooth/hci.h>
// #include <bluetooth/hci_lib.h>
// #include <stdlib.h>
// #include <unistd.h>
// #include "bt_linux.h"
import "C"

var (
	// MaxLEAdvertisingPayloadSize denotes the maximum size for the
	// LE advertising payload.
	// See the Bluetooth 4.0 spec for more info on Bluetooth LE payload
	// structure:
	// https://www.bluetooth.org/en-us/specification/adopted-specifications
	MaxLEAdvertisingPayloadSize = int(C.kMaxLEPayloadSize)

	// MaxChannel represents the highest channel id that can be used
	// for establishing RFCOMM connections.
	MaxChannel = int(C.kMaxChannel)

	// maxDevices denotes a maximum number of local devices to scan over
	// when a particular device isn't explicitly specified (e.g.,
	// OpenFirstAvailableDevice).
	maxDevices = int(C.kMaxDevices)
)

// OpenDevice opens the Bluetooth device with a specified id, returning an
// error if the device couldn't be opened, or nil otherwise.
func OpenDevice(deviceID int) (*Device, error) {
	if deviceID < 0 {
		return nil, fmt.Errorf("negative device id, use OpenFirstAvailableDevice() instead")
	}
	var descriptor C.int
	var name *C.char
	var localMAC *C.char
	if es := C.bt_open_device(C.int(deviceID), &descriptor, &name, &localMAC); es != nil {
		defer C.free(unsafe.Pointer(es))
		return nil, fmt.Errorf("error opening device %d: %s", deviceID, C.GoString(es))
	}
	defer C.free(unsafe.Pointer(name))
	defer C.free(unsafe.Pointer(localMAC))
	mac, err := net.ParseMAC(C.GoString(localMAC))
	if err != nil {
		return nil, fmt.Errorf("illegal hardware address %q for device %d: %s", C.GoString(localMAC), deviceID, err)
	}
	return &Device{
		Name:       C.GoString(name),
		MAC:        mac,
		id:         deviceID,
		descriptor: int(descriptor),
	}, nil
}

// OpenFirstAvailableDevice() opens the first available bluetooth device,
// returning an error if no device could be opened, or nil otherwise.
func OpenFirstAvailableDevice() (*Device, error) {
	for devID := 0; devID < maxDevices; devID++ {
		if d, err := OpenDevice(devID); err == nil {
			return d, nil
		}
	}
	return nil, fmt.Errorf("can't find an available bluetooth device")
}

// Listen creates a new listener for RFCOMM connections on the provided
// local address, specified in the <MAC-Channel> format (e.g.,
// "01:23:45:67:89:AB-1").  Channel number 0 means pick the first available
// channel.  Empty MAC address means pick the first available bluetooth device.
// Error is returned if a listener cannot be created.
// Note that the returned net.Listener won't use the runtime network poller
// and hence a new OS thread will be created for every outstanding connection.
func Listen(localAddr string) (net.Listener, error) {
	local, err := parseAddress(localAddr)
	if err != nil {
		return nil, fmt.Errorf("listen error: invalid local address format %s, error: %s", localAddr, err)
	}
	if local.channel > MaxChannel {
		return nil, fmt.Errorf("listen error: channel %d too large - max: %d", local.channel, MaxChannel)
	}

	// Open a new local bluetooth socket.
	socket := C.socket(C.AF_BLUETOOTH, C.SOCK_STREAM, C.BTPROTO_RFCOMM)
	if socket < 0 {
		return nil, fmt.Errorf("listen error: can't open new RFCOMM socket")
	}

	// Bind to the local socket.
	var localMAC *C.char
	if !local.isAnyMAC() {
		localMAC = C.CString(local.mac.String())
	}
	defer C.free(unsafe.Pointer(localMAC))
	localChannel := C.int(local.channel)
	if es := C.bt_bind(socket, &localMAC, &localChannel); es != nil {
		defer C.free(unsafe.Pointer(es))
		syscall.Close(int(socket))
		return nil, fmt.Errorf("listen error: %v", C.GoString(es))
	}
	// Re-parse the address as it may have changed.
	if local.mac, err = net.ParseMAC(C.GoString(localMAC)); err != nil {
		syscall.Close(int(socket))
		return nil, fmt.Errorf("listen error: invalid local MAC address: %s, err: %v", C.GoString(localMAC), err)
	}
	local.channel = int(localChannel)

	// Create a listener for incoming connections.
	const maxPendingConnections = 100
	if err = syscall.Listen(int(socket), maxPendingConnections); err != nil {
		syscall.Close(int(socket))
		return nil, fmt.Errorf("listen error: %v", err)
	}
	return newListener(int(socket), local)
}

// Dial creates a new RFCOMM connection with the remote address, specified in
// the <MAC/Channel> format (e.g., "01:23:45:67:89:AB-1").  It returns an error
// if the connection cannot be established.
// Note that the returned net.Conn won't use the runtime network poller and
// hence a new OS thread will be created for every outstanding connection.
func Dial(remoteAddr string) (net.Conn, error) {
	remote, err := parseAddress(remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("dial error: invalid remote address format %s, error %s", remoteAddr, err)
	}
	if remote.isAnyMAC() {
		return nil, fmt.Errorf("dial error: must specify remote MAC address: %s", remoteAddr)
	}
	if remote.channel > MaxChannel {
		return nil, fmt.Errorf("dial error: channel %d too large - max: %d", remote.channel, MaxChannel)
	}

	// Open a new local bluetooth socket.
	socket := C.socket(C.AF_BLUETOOTH, C.SOCK_STREAM, C.BTPROTO_RFCOMM)
	if socket < 0 {
		return nil, fmt.Errorf("dial error: can't open new RFCOMM socket")
	}

	// Bind to the local socket.
	var localMAC *C.char     // bind to first available device
	localChannel := C.int(0) // bind to first available channel
	if es := C.bt_bind(socket, &localMAC, &localChannel); es != nil {
		defer C.free(unsafe.Pointer(es))
		syscall.Close(int(socket))
		return nil, fmt.Errorf("dial error: %v", C.GoString(es))
	}
	defer C.free(unsafe.Pointer(localMAC))
	// Parse the local address.
	var local addr
	if local.mac, err = net.ParseMAC(C.GoString(localMAC)); err != nil {
		return nil, fmt.Errorf("dial error: invalid local MAC address: %s, err: %s", C.GoString(localMAC), err)
	}
	local.channel = int(localChannel)

	// Connect to the remote address.
	remoteMAC := C.CString(remote.mac.String())
	defer C.free(unsafe.Pointer(remoteMAC))
	remoteChannel := C.int(remote.channel)
	if es := C.bt_connect(socket, remoteMAC, remoteChannel); es != nil {
		defer C.free(unsafe.Pointer(es))
		return nil, fmt.Errorf("dial error: error connecting to remote address: %s, error: %s", remoteAddr, C.GoString(es))
	}
	return newConn(int(socket), &local, remote)
}

func (d *Device) String() string {
	return fmt.Sprintf("BT_DEVICE(%s, %v)", d.Name, d.MAC)
}

// StartAdvertising starts LE advertising on the Bluetooth device, sending
// one advertising packet after every tick of the provided time interval.
// The payload sent with each advertising packet can be specified via the
// SetAdvertisingPayload method.
// This method may be called again even if advertising is currently enabled,
// in order to adjust the advertising interval.
func (d *Device) StartAdvertising(interval time.Duration) error {
	if es := C.bt_start_le_advertising(C.int(d.descriptor), C.int(int64(interval/time.Millisecond))); es != nil {
		defer C.free(unsafe.Pointer(es))
		return fmt.Errorf("error starting LE advertising on device: %v, error: %s", d, C.GoString(es))
	}
	return nil
}

// SetAdvertisingPayload sets the advertising payload that is sent with each
// advertising packet.  This function may be called at any time to adjust the
// payload that is currently being advertised.
func (d *Device) SetAdvertisingPayload(payload string) error {
	if es := C.bt_set_le_advertising_payload(C.int(d.descriptor), C.CString(payload)); es != nil {
		defer C.free(unsafe.Pointer(es))
		return fmt.Errorf("error setting advertising payload on device: %v, error: %s", d, C.GoString(es))
	}
	return nil
}

// StopAdvertising stops LE advertising on the Bluetooth device.  If the
// device is not advertising, this function will be a noop.
func (d *Device) StopAdvertising() error {
	if es := C.bt_stop_le_advertising(C.int(d.descriptor)); es != nil {
		defer C.free(unsafe.Pointer(es))
		return fmt.Errorf("error stopping LE advertising on device: %v, error: %s", d, C.GoString(es))
	}
	return nil
}

// StartScan initiates a Low-Energy scan on the Bluetooth device.  The scan
// will proceed over many duration intervals; within each interval, scan will
// be ON only for a given duration window.  All scan readings encountered
// during scan-ON periods are pushed onto the returned channel.  If the scan
// cannot be started, an error is returned.
func (d *Device) StartScan(scanInterval, scanWindow time.Duration) (<-chan ScanReading, error) {
	if scanInterval < scanWindow {
		return nil, fmt.Errorf("invalid scan settings: scan interval %d must be greater or equal to scan window %d", scanInterval, scanWindow)
	}
	// Set scan params.
	const (
		passiveScan      = 0x00
		publicAddress    = 0x00
		acceptAllPackets = 0x00
		timeoutMS        = 1000
	)
	if ret, err := C.hci_le_set_scan_parameters(C.int(d.descriptor), passiveScan, C.uint16_t(scanInterval/time.Millisecond), C.uint16_t(scanWindow/time.Millisecond), publicAddress, acceptAllPackets, timeoutMS); ret < 0 {
		return nil, fmt.Errorf("error setting LE scan parameters: %v", err)
	}
	// Enable scan.
	const (
		scanEnabled               = 0x01
		disableDuplicateFiltering = 0x00
	)
	if ret, err := C.hci_le_set_scan_enable(C.int(d.descriptor), scanEnabled, disableDuplicateFiltering, timeoutMS); ret < 0 {
		return nil, fmt.Errorf("error enabling LE scan: %v", err)
	}
	// Set the event filter options.  We're only interested in LE meta
	// events.
	var fopts C.struct_hci_filter
	C.hci_filter_clear(&fopts)
	C.hci_filter_set_ptype(C.HCI_EVENT_PKT, &fopts)
	C.hci_filter_set_event(C.EVT_LE_META_EVENT, &fopts)
	if ret, err := C.setsockopt(C.int(d.descriptor), C.SOL_HCI, C.HCI_FILTER, unsafe.Pointer(&fopts), C.socklen_t(unsafe.Sizeof(fopts))); ret < 0 {
		return nil, fmt.Errorf("couldn't set filter options on socket: %v", err)
	}

	// Start the reading go-routine.
	d.leScanChan = make(chan ScanReading, 10)
	go d.leScanLoop()
	return d.leScanChan, nil
}

func (d *Device) leScanLoop() {
	defer vlog.Info("LE scan reading goroutine exiting")
	buf := make([]byte, C.HCI_MAX_EVENT_SIZE)
	for {
		// Read one advertising meta event.
		n, err := syscall.Read(int(C.int(d.descriptor)), buf)
		if err != nil || n < 0 {
			vlog.Errorf("error getting scan readings: %v", err)
			return
		}
		// Get data of interest to us.
		var remoteMAC, remoteName *C.char
		var rssi, done C.int
		if es := C.bt_parse_le_meta_event(unsafe.Pointer(&buf[0]), &remoteMAC, &remoteName, &rssi, &done); es != nil {
			vlog.Errorf("couldn't parse LE meta event: %s", C.GoString(es))
			C.free(unsafe.Pointer(es))
			continue
		}
		if done != 0 { // Scan stopped.
			return
		}
		name := C.GoString(remoteName)
		C.free(unsafe.Pointer(remoteName))
		mac, err := net.ParseMAC(C.GoString(remoteMAC))
		C.free(unsafe.Pointer(remoteMAC))
		if err != nil {
			vlog.Errorf("invalid MAC address: %v", mac)
			continue
		}
		d.leScanChan <- ScanReading{
			Name:     name,
			MAC:      mac,
			Distance: distanceFromRSSI(int(rssi)),
			Time:     time.Now(),
		}
	}
}

// StopScan stops any Low-Energy scan in progress on the Bluetooth device.
// If the device is not scanning, this function will be a noop.
func (d *Device) StopScan() error {
	// Disable scan.  This will also stop the reading goroutine.
	const (
		scanDisabled              = 0x00
		disableDuplicateFiltering = 0x00
		timeoutMS                 = 1000
	)
	if ret, err := C.hci_le_set_scan_enable(C.int(d.descriptor), scanDisabled, disableDuplicateFiltering, timeoutMS); ret < 0 {
		return fmt.Errorf("error disabling LE scan: %v", err)
	}

	close(d.leScanChan)
	return nil
}

// Close closes our handle on the Bluetooth device.  Note that this call doesn't
// stop any operations in progress on the device (e.g., LE advertising) - it
// simply closes the handle to it.
func (d *Device) Close() error {
	if es := C.bt_close_device(C.int(d.descriptor)); es != nil {
		defer C.free(unsafe.Pointer(es))
		return fmt.Errorf("error closing device %v, error: %s", d, C.GoString(es))
	}
	return nil
}

// distanceFromRSSI computes the distance to the neighboring device using
// the RSSI of that device's advertising packet.
func distanceFromRSSI(rssi int) unit.Distance {
	// We're using the formula (and observed constants) from the following
	// paper:
	//   "Outdoor Localization System Using RSSI Measurement of Wireless
	//    Sensor Network"
	//   by: Oguejiofor O.S., Okorogu V.N., Adewale Abe, and Osuesu B.O
	//   link: http://www.ijitee.org/attachments/File/v2i2/A0359112112.pdf
	//
	// Formula:
	//
	//       RSSI [dBm] = -10n log10(d [m]) + A [dBm]
	//
	//, where:
	//
	//       A = received signal strength at 1m (observed to be -44.8dBm)
	//       n = propagation pathloss exponent (observed to be 2.2)
	//       d = distance (in meters)
	//
	// The final formula for distance (in meters) therefore comes down to:
	//
	//       d = 10^((RSSI / -22) - 2.036)
	//
	return unit.Distance(math.Pow(10.0, (float64(rssi)/-22.0)-2.036)) * unit.Meter
}

// +build veyronbluetooth,!android

#include <stdlib.h>

// Maximum allowed LE payload size. See the Bluetooth 4.0 spec for more info on
// Bluetooth LE payload structure:
//   https://www.bluetooth.org/en-us/specification/adopted-specifications
const int kMaxLEPayloadSize;
// The highest bluetooth channel that can be used for establishing RFCOMM
// connections.
const int kMaxChannel;

// Maximum number of local devices to scan over when a particular device isn't
// explicitly specified.
const int kMaxDevices;

// Opens the bluetooth device with the provided id, storing its device
// descriptor into '*dd', its device name into '*name', and its MAC address
// into '*local_address'.
// Returns an error string if any error is encoutered.  If a non-NULL error
// string is returned, the caller must free it.
// The caller must free '*name' and '*local_address' strings whenever a NULL
// error string is returned.
// REQUIRES: dev_id >= 0
char* bt_open_device(int dev_id, int* dd, char** name, char** local_address);

// Closes the (previously opened) device with the given device descriptor.
// Returns error string on failure, or NULL otherwise.  If a non-NULL error
// string is returned, the caller must free it.
char* bt_close_device(int dd);

// Binds the given socket to the provided MAC address/channel.  If '*local_address'
// is NULL, it will bind to the first available bluetooth device and overwrite
// '*local_address' to contain that device's MAC address.  If '*channel' is zero,
// it will bind to the first available channel on a given device and overwrite
// '*channel' to contain that channel value.  (If both of the above are true, we will
// find the first device/channel pair that works and overwrite both values.)
// Returns an error string if any error is encoutered.  If a non-NULL error
// string is returned, the caller must free it.
// The caller must free '*local_address' string whenever a NULL value was passed
// in and a NULL error string is returned.
char* bt_bind(int sock, char** local_address, int* channel);

// Accepts the next connection on the provided socket.  Stores the file
// descriptor for the newly established connection into 'fd', and the MAC
// address of the remote party into 'remote_address".  Returns an error string
// if any error is encountered.  If a non-NULL error string is returned, the
// caller must free it.
// The caller must free '*remote_address' string whenever a NULL error string
// is returned.
char* bt_accept(int sock, int* fd, char** remote_address);

// Connects to the remote address/channel pair, using the provided local socket.
// Returns an error string if any error is encountered.  If a non-NULL error
// string is returned, the caller must free it.
char* bt_connect(int sock, const char* remote_address, int remote_channel);

// Starts bluetooth LE advertising on the provided device descriptor, sending
// one advertising packet every 'adv_interval_ms' milliseconds.
// Returns error string on failure, or NULL otherwise.  If a non-NULL error
// string is returned, the caller must free it.
char* bt_start_le_advertising(int dd, int adv_interval_ms);

// Sets the advertising payload that is sent with each advertising packet.
// This function may be called at any time to adjust the payload that is
// currently being advertised.
// Returns error string on failure, or NULL otherwise.  If a non-NULL error
// string is returned, the caller must free it.
char* bt_set_le_advertising_payload(int dd, char* adv_payload);

// Stops bluetooth LE advertising on the provided device descriptor.
// Returns error string on failure, or NULL otherwise.  If a non-NULL error
// string is returned, the caller must free it.
char* bt_stop_le_advertising(int dd);

// Parses the LE meta event, extracting remote address, name, and RSSI.  It also
// checks whether the event is "LE Connection Complete Event", which indicates
// that the scan has stopped, and writes the result of that check into 'done'.
// Returns error if data cannot be parsed.  If a non-NULL error string
// is returned, the caller must free it.
char* bt_parse_le_meta_event(
    void* data, char** remote_addr, char** remote_name, int* rssi, int* done);

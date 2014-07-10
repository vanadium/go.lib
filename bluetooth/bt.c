// +build linux,!android

#include "bt.h"

#include <assert.h>
#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/ioctl.h>
#include <sys/select.h>
#include <sys/socket.h>
#include <bluetooth/bluetooth.h>
#include <bluetooth/rfcomm.h>
#include <bluetooth/hci.h>
#include <bluetooth/hci_lib.h>

#define EIR_FLAGS                   0x01  /* flags */
#define EIR_UUID16_SOME             0x02  /* 16-bit UUID, more available */
#define EIR_UUID16_ALL              0x03  /* 16-bit UUID, all listed */
#define EIR_UUID32_SOME             0x04  /* 32-bit UUID, more available */
#define EIR_UUID32_ALL              0x05  /* 32-bit UUID, all listed */
#define EIR_UUID128_SOME            0x06  /* 128-bit UUID, more available */
#define EIR_UUID128_ALL             0x07  /* 128-bit UUID, all listed */
#define EIR_NAME_SHORT              0x08  /* shortened local name */
#define EIR_NAME_COMPLETE           0x09  /* complete local name */
#define EIR_TX_POWER                0x0A  /* transmit power level */
#define EIR_DEVICE_ID               0x10  /* device ID */

// Timeout for all hci requests, in milliseconds.
static const int kTimeoutMs = 1000;
static const int kMaxAddrStrSize = 18;

const int kMaxLEPayloadSize = 26;
const int kMaxChannel = 30;
const int kMaxDevices = 5;

char* bt_open_device(int dev_id, int* dd, char** name, char** local_address) {
  char* err = NULL;
  int sock;
  struct hci_dev_req dev_req;
  struct hci_dev_info di;
  bdaddr_t loc_addr;

  if (dev_id < 0) {
    asprintf(err, "can't pass negative device id for bt_open_device().");
    return err;
  }

  // Open HCI socket.
  if ((sock = socket(AF_BLUETOOTH, SOCK_RAW, BTPROTO_HCI)) < 0) {
    asprintf(&err, "can't open HCI socket:%d[%s]", errno, strerror(errno));
    return err;
  }

  // Get device's name.
  di.dev_id = dev_id;
  if (ioctl(sock, HCIGETDEVINFO, (void *)&di) < 0) {
    asprintf(&err, "can't get device info:%d[%s]", errno, strerror(errno));
    close(sock);
    return err;
  }
  *name = (char*) malloc(strlen(di.name) * sizeof(char));
  strcpy(*name, di.name);

  // Try to open the specified device.
  ioctl(sock, HCIDEVUP, dev_id);
  *dd = hci_open_dev(dev_id);
  if (*dd < 0) {
    asprintf(&err, "can't open device %d:%d[%s]", dev_id, errno, strerror(errno));
    close(sock);
    return err;
  }

  // NOTE(spetrovic): We need to enable page scanning on the device for
  // RFCOMM connections to work.  Since this requires root access, it will
  // probably need to be done elsewhere (e.g., 'sudo hciconfig hci0 pscan').

  // Get device's local MAC address.
  hci_devba(dev_id, &loc_addr);
  *local_address = (char*) malloc(kMaxAddrStrSize * sizeof(char));
  ba2str(&loc_addr, *local_address);

  return NULL;
}

char* bt_bind(int sock, char** local_address, int* channel) {
  char* err = NULL;
  int dev_id, dd;
  struct sockaddr_rc addr = { 0 };

  if (*local_address == NULL) {
    char* name = NULL;
    // Find the first available device.
    for (dev_id = 0; dev_id < kMaxDevices; dev_id++) {
      if ((err = bt_open_device(dev_id, &dd, &name, local_address)) != NULL) {
        free(err);
        continue;
      }
      bt_close_device(dd);
      assert(*local_address != NULL);
      if ((err = bt_bind(sock, local_address, channel)) != NULL) {
        free(err);
        free(*local_address);
        continue;
      }
      return NULL;
    }
    asprintf(&err, "can't find an available bluetooth device");
    return err;
  } else if (*channel == 0) {
    // Find the first available channel.
    for (*channel = 1; *channel < kMaxChannel; (*channel)++) {
      if ((err = bt_bind(sock, local_address, channel)) != NULL) {
        free(err);
        continue;
      }
      return NULL;
    }
    asprintf(&err, "can't find an available bluetooth channel");
    return err;
  } else {  // *local_address != NULL && *channel > 0
    addr.rc_family = AF_BLUETOOTH;
    str2ba(*local_address, &addr.rc_bdaddr);
    addr.rc_channel = (uint8_t) *channel;
    if (bind(sock, (struct sockaddr*) &addr, sizeof(addr)) < 0) {
      asprintf(&err, "can't bind to socket %d, addr %s, channel %d, error: %d[%s]",
               sock, *local_address, *channel, errno, strerror(errno));
      return err;
    }
    return NULL;
  }
}

char* bt_accept(int sock, int* fd, char** remote_address) {
  char* err = NULL;
  struct sockaddr_rc remote_addr;
  socklen_t opt = sizeof(remote_addr);

  if ((*fd = accept(sock, (struct sockaddr *)&remote_addr, &opt)) < 0) {
    asprintf(&err, "error accepting connection on socket %d, error: %d[%s]",
             sock, errno, strerror(errno));
    return err;
  }
  *remote_address = (char*) malloc(kMaxAddrStrSize * sizeof(char));
  ba2str(&remote_addr.rc_bdaddr, *remote_address);

  return NULL;
}

char* bt_connect(int sock, const char* remote_address, int remote_channel) {
  char* err = NULL;
  struct sockaddr_rc remote_addr = { 0 };

  remote_addr.rc_family = AF_BLUETOOTH;
  str2ba(remote_address, &remote_addr.rc_bdaddr);
  remote_addr.rc_channel = (uint8_t) remote_channel;
  if (connect(sock, (struct sockaddr*) &remote_addr, sizeof(remote_addr)) < 0) {
    asprintf(&err, "can't connect to remote address %s and channel %d "
             "on socket %d: %d[%s]", remote_address, remote_channel,
             sock, errno, strerror(errno));
    return err;
  }
  return NULL;
}

char* bt_close_device(int dd) {
  char* err = NULL;
  if (hci_close_dev(dd) < 0) {
    asprintf(&err, "can't close device with dd: %d, error: %d[%s]", dd, errno, strerror(errno));
    return err;
  }
  return NULL;
}

char* bt_enable_le_advertising(int dd, int enable) {
  char* err = NULL;
  struct hci_request req;
  le_set_advertise_enable_cp adv_enable_cp;
  uint8_t status;

  memset(&adv_enable_cp, 0, sizeof(adv_enable_cp));
  adv_enable_cp.enable = enable;

  memset(&req, 0, sizeof(req));
  req.ogf = OGF_LE_CTL;
  req.ocf = OCF_LE_SET_ADVERTISE_ENABLE;
  req.cparam = &adv_enable_cp;
  req.clen = LE_SET_ADVERTISE_ENABLE_CP_SIZE;
  req.rparam = &status;
  req.rlen = 1;

  if (hci_send_req(dd, &req, kTimeoutMs) < 0) {
    asprintf(&err,
             "can't enable/disable advertising for dd: %d, status: %d, error: %d",
             dd, status, errno);
    return err;
  }
  return NULL;
}

char* bt_start_le_advertising(int dd, int adv_interval_ms) {
  char* err = NULL;
  struct hci_request req;
  le_set_advertising_parameters_cp adv_params_cp;
  le_set_advertise_enable_cp adv_enable_cp;
  uint8_t status;

  // Set advertising params.
  memset(&adv_params_cp, 0, sizeof(adv_params_cp));
  adv_params_cp.min_interval = adv_interval_ms;
  adv_params_cp.max_interval = adv_interval_ms;
  adv_params_cp.advtype = 0x00;  // Connectable undirected advertising.
  adv_params_cp.chan_map = 7;

  memset(&req, 0, sizeof(req));
  req.ogf = OGF_LE_CTL;
  req.ocf = OCF_LE_SET_ADVERTISING_PARAMETERS;
  req.cparam = &adv_params_cp;
  req.clen = LE_SET_ADVERTISING_PARAMETERS_CP_SIZE;
  req.rparam = &status;
  req.rlen = 1;

  if (hci_send_req(dd, &req, kTimeoutMs) < 0) {
    asprintf(&err,
             "can't set advertising params for dd: %d, status: %d, error: %d",
             dd, status, errno);
    return err;
  }

  // Start advertising.
  return bt_enable_le_advertising(dd, 1);
}

char* bt_set_le_advertising_payload(int dd, char* adv_payload) {
  char* err = NULL;
  int idx;
  struct hci_request req;
  le_set_advertising_data_cp adv_data_cp;
  uint8_t status;

  if (strlen(adv_payload) > kMaxLEPayloadSize) {
    asprintf(&err, "payload too big");
    return err;
  }

  // Set advertising data.
  memset(&adv_data_cp, 0, sizeof(adv_data_cp));
  idx = 0;
  adv_data_cp.data[idx++] = 2;
  adv_data_cp.data[idx++] = EIR_FLAGS;
  adv_data_cp.data[idx++] = 0x06;  // general discoverable+BR/EDR Not Supported
  adv_data_cp.data[idx++] = strlen(adv_payload) + 1;
  adv_data_cp.data[idx++] = EIR_NAME_COMPLETE;
  memcpy(&adv_data_cp.data[idx], adv_payload, strlen(adv_payload));
  idx += strlen(adv_payload);
  adv_data_cp.length = idx;

  memset(&req, 0, sizeof(req));
  req.ogf = OGF_LE_CTL;
  req.ocf = OCF_LE_SET_ADVERTISING_DATA;
  req.cparam = &adv_data_cp;
  req.clen = LE_SET_ADVERTISING_DATA_CP_SIZE;
  req.rparam = &status;
  req.rlen = 1;

  if (hci_send_req(dd, &req, kTimeoutMs) < 0) {
    asprintf(&err,
             "can't set advertising data for dd: %d, status: %d, error: %d\n",
             dd, status, errno);
    return err;
  }
  return NULL;
}

char* bt_stop_le_advertising(int dd) {
  return bt_enable_le_advertising(dd, 0);
}

char* bt_parse_le_meta_event(
    void* data, char** remote_addr, char** remote_name, int* rssi, int* done) {
  char *err, *ptr, *end;
  evt_le_meta_event* meta_event;
  le_advertising_info* adv_info;

  *done = 0;
  meta_event = (evt_le_meta_event*) (data + 1 + HCI_EVENT_HDR_SIZE);
  if (meta_event->subevent == 0x01 /* LE Connection Complete Event */) {
    // This event is triggered when scan is disabled.
    *done = 1;
    return NULL;
  } else if (meta_event->subevent != 0x02 /* LE Advertising Report Event */) {
    asprintf(&err, "wrong event type: %d", meta_event->subevent);
    return err;
  }

  adv_info = (le_advertising_info*) (meta_event->data + 1);
  *remote_addr = malloc(kMaxAddrStrSize * sizeof(char));
  ba2str(&adv_info->bdaddr, *remote_addr);
  *rssi = *((int8_t*) adv_info->data + adv_info->length);

  // Extract name.
  // Max possible advertising data length, as defined by the standard.
  // The actual maximum name length was observed to be less than that, but
  // this value will do.
  const int kMaxNameLength = 31;
  *remote_name = malloc(kMaxNameLength * sizeof(char));
  memset(*remote_name, 0, kMaxNameLength);
  ptr = adv_info->data;
  end = adv_info->data + adv_info->length;

  // Go through all advertising data packets in the response.
  while (ptr < end) {
    int adv_data_length = *(ptr++);  // includes adv_data_type below.
    int adv_data_type;
    if (adv_data_length == 0) {  // end of response.
      break;
    } else if ((ptr + adv_data_length) > end) {
      // Illegal adv_data length.
      break;
    }
    adv_data_type = *(ptr++);
    switch (adv_data_type) {
      case EIR_NAME_SHORT:
      case EIR_NAME_COMPLETE:
        if (adv_data_length - 1 > kMaxNameLength) {
          // Illegal name length.
          break;
        }
        memcpy(*remote_name, ptr, adv_data_length - 1);
        break;
    }
    ptr += adv_data_length - 1;
  }
  return NULL;
}

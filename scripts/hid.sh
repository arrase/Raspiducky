#!/bin/bash

. /etc/raspiducky/raspiducky.conf

cd /sys/kernel/config/usb_gadget/
mkdir -p g1
cd g1
echo 0x1d6b > idVendor # Linux Foundation
echo 0x0104 > idProduct # Multifunction Composite Gadget
echo 0x0100 > bcdDevice # v1.0.0
echo 0x0200 > bcdUSB # USB2
mkdir -p strings/0x409
echo "fedcba9876543210" > strings/0x409/serialnumber
echo "Parasite Team" > strings/0x409/manufacturer
echo "Raspiducky" > strings/0x409/product
N="usb0"

# KEYBOARD
mkdir -p functions/hid.$N
echo 1 > functions/hid.usb0/protocol
echo 1 > functions/hid.usb0/subclass
echo 8 > functions/hid.usb0/report_length
echo -ne \\x05\\x01\\x09\\x06\\xa1\\x01\\x05\\x07\\x19\\xe0\\x29\\xe7\\x15\\x00\\x25\\x01\\x75\\x01\\x95\\x08\\x81\\x02\\x95\\x01\\x75\\x08\\x81\\x03\\x95\\x05\\x75\\x01\\x05\\x08\\x19\\x01\\x29\\x05\\x91\\x02\\x95\\x01\\x75\\x03\\x91\\x03\\x95\\x06\\x75\\x08\\x15\\x00\\x25\\x65\\x05\\x07\\x19\\x00\\x29\\x65\\x81\\x00\\xc0 > functions/hid.usb0/report_desc
C=1
mkdir -p configs/c.$C/strings/0x409
echo "Config $C: ECM network" > configs/c.$C/strings/0x409/configuration
echo 250 > configs/c.$C/MaxPower
ln -s functions/hid.$N configs/c.$C/
# End KEYBOARD

# STORAGE
if [ $STORAGE_MODE != "none" ]
then
    mkdir -p functions/mass_storage.usb0
    echo 1 > functions/mass_storage.usb0/stall
    echo 0 > functions/mass_storage.usb0/lun.0/removable
    echo 0 > functions/mass_storage.usb0/lun.0/cdrom
    echo 0 > functions/mass_storage.usb0/lun.0/ro
    echo 0 > functions/mass_storage.usb0/lun.0/nofua

    if [ $STORAGE_MODE = "disk" ]
    then
        [ -d $STORAGE_MOUNT ] || mkdir $STORAGE_MOUNT
        mount -o loop,rw -t vfat $STORAGE_FILE $STORAGE_MOUNT
        echo $STORAGE_FILE > functions/mass_storage.usb0/lun.0/file
    else
        echo $CONFIG_DISK > functions/mass_storage.usb0/lun.0/file
    fi

    ln -s functions/mass_storage.usb0 configs/c.$C/
fi
# End STORAGE

ls /sys/class/udc > UDC


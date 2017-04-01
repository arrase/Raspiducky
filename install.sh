#!/bin/bash

INSTALL_DIR=/home/pi

gcc hid-gadget-test.c -o $INSTALL_DIR/hid-gadget-test
cp usleep $INSTALL_DIR/
cp duckpi.sh $INSTALL_DIR/
cp hid.sh $INSTALL_DIR/
cp run_payload.sh $INSTALL_DIR

chmod 777 $INSTALL_DIR/hid-gadget-test
chmod 777 $INSTALL_DIR/usleep
chmod 777 $INSTALL_DIR/duckpi.sh
chmod 777 $INSTALL_DIR/hid.sh
chmod 777 $INSTALL_DIR/run_payload.sh

[ -d /etc/raspiducky ] || sudo mkdir /etc/raspiducky
[ -f /etc/raspiducky/raspiducky.conf ] || sudo cp raspiducky.conf /etc/raspiducky/raspiducky.conf

sudo echo "dtoverlay=dwc2" >> /boot/config.txt
sudo echo "dwc2" >> /etc/modules
sudo echo "libcomposite" >> /etc/modules

cat /etc/rc.local | awk '/exit\ 0/ && c == 0 {c = 0; print "\n/home/pi/hid.sh\nsleep 3\n/home/pi/run_payload.sh\n"}; {print}' /etc/rc.local

if ! [ -e /home/pi/usbdisk.img ]
then
    dd if=/dev/zero of=/home/pi/usbdisk.img bs=1024 count=10000
    mkfs.vfat /home/pi/usbdisk.img
fi

#!/bin/bash

INSTALL_DIR=/home/pi
FLASH_DISK_SIZE=100000 # 100MB

# EXEC FILES

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

# APP CONFIG

dd if=/dev/zero of=$INSTALL_DIR/.confdisk.img bs=1024 count=10000
mkfs.vfat $INSTALL_DIR/.confdisk.img

[ -d $INSTALL_DIR/ ] || mkdir $INSTALL_DIR/config
sudo mount $INSTALL_DIR/.confdisk.img $INSTALL_DIR/config -o loop,rw
sudo echo "$INSTALL_DIR/.confdisk.img  $INSTALL_DIR/config           vfat    defaults          0       2"

[ -d $INSTALL_DIR/config/etc ] || sudo mkdir $INSTALL_DIR/config/etc
[ -f $INSTALL_DIR/config/etc/raspiducky.conf ] || sudo cp raspiducky.conf $INSTALL_DIR/config/etc/raspiducky.conf
[ -d $INSTALL_DIR/config/payloads-db ] || cp -r payloads $INSTALL_DIR/config/payloads-db
[ -d $INSTALL_DIR/config/onboot_payload ] || mkdir $INSTALL_DIR/config/onboot_payload

# BOOT CONFIG

sudo echo "dtoverlay=dwc2" >> /boot/config.txt
sudo echo "dwc2" >> /etc/modules
sudo echo "libcomposite" >> /etc/modules

cat /etc/rc.local | awk '/exit\ 0/ && c == 0 {c = 0; print "\n/home/pi/hid.sh\nsleep 3\n/home/pi/run_payload.sh\n"}; {print}' /etc/rc.local

# FLASH DRIVE

dd if=/dev/zero of=$INSTALL_DIR/.usbdisk.img bs=1024 count=$FLASH_DISK_SIZE
mkfs.vfat $INSTALL_DIR/.usbdisk.img

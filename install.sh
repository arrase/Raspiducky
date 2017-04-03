#!/bin/bash

INSTALL_DIR=/home/pi
USERID=1000
GROUPID=1000
FLASH_DISK_SIZE=100000 # 100MB

# EXEC FILES

gcc hid-gadget-test.c -o $INSTALL_DIR/hid-gadget-test
cp hid.sh $INSTALL_DIR/
cp run_payload.sh $INSTALL_DIR
cp raspiducky.py $INSTALL_DIR

chmod 777 $INSTALL_DIR/hid-gadget-test
chmod 777 $INSTALL_DIR/raspiducky.py
chmod 777 $INSTALL_DIR/hid.sh
chmod 777 $INSTALL_DIR/run_payload.sh

# APP CONFIG

dd if=/dev/zero of=$INSTALL_DIR/.confdisk.img bs=1024 count=10000
mkfs.vfat $INSTALL_DIR/.confdisk.img

[ -d $INSTALL_DIR/config ] || mkdir $INSTALL_DIR/config
sudo mount $INSTALL_DIR/.confdisk.img $INSTALL_DIR/config -o loop,rw,uid=$USERID,gid=$GROUPID

[ -d $INSTALL_DIR/config/etc ] || mkdir $INSTALL_DIR/config/etc
[ -f $INSTALL_DIR/config/etc/raspiducky.conf ] || cp raspiducky.conf $INSTALL_DIR/config/etc/raspiducky.conf
[ -d $INSTALL_DIR/config/payloads-db ] || cp -r payloads $INSTALL_DIR/config/payloads-db
[ -d $INSTALL_DIR/config/onboot_payload ] || mkdir $INSTALL_DIR/config/onboot_payload
echo "$INSTALL_DIR/.confdisk.img   $INSTALL_DIR/config    vfat    loop,rw          0       2" | sudo tee --append /etc/fstab
sudo umount $INSTALL_DIR/config

# BOOT CONFIG

echo "dtoverlay=dwc2" | sudo tee --append /boot/config.txt
echo "dwc2" | sudo tee --append /etc/modules
echo "libcomposite" | sudo tee --append /etc/modules

cat /etc/rc.local | sudo awk '/exit\ 0/ && c == 0 {c = 0; print "\n/home/pi/hid.sh\nsleep 3\n/home/pi/run_payload.sh\n"}; {print}' /etc/rc.local

# FLASH DRIVE

dd if=/dev/zero of=$INSTALL_DIR/.usbdisk.img bs=1024 count=$FLASH_DISK_SIZE
mkfs.vfat $INSTALL_DIR/.usbdisk.img

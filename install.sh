#!/bin/bash

INSTALL_DIR=/home/pi
ETC_DIR=/etc/raspiducky
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

[ -d $ETC_DIR ] || sudo mkdir $ETC_DIR
sudo mount $INSTALL_DIR/.confdisk.img $ETC_DIR -o loop,rw

[ -f $ETC_DIR/raspiducky.conf ] || sudo cp raspiducky.conf $ETC_DIR/raspiducky.conf
[ -d $ETC_DIR/payloads-db ] || sudo cp -r payloads $ETC_DIR/payloads-db
[ -d $ETC_DIR/onboot_payload ] || sudo mkdir $ETC_DIR/onboot_payload
echo "$INSTALL_DIR/.confdisk.img   $ETC_DIR    vfat    loop,rw          0       2" | sudo tee --append /etc/fstab
sudo umount $ETC_DIR

# BOOT CONFIG

echo "dtoverlay=dwc2" | sudo tee --append /boot/config.txt
echo "dwc2" | sudo tee --append /etc/modules
echo "libcomposite" | sudo tee --append /etc/modules

cat /etc/rc.local | sudo awk '/exit\ 0/ && c == 0 {c = 0; print "\n/home/pi/hid.sh\nsleep 3\n/home/pi/run_payload.sh\n"}; {print}' /etc/rc.local

# FLASH DRIVE

dd if=/dev/zero of=$INSTALL_DIR/.usbdisk.img bs=1024 count=$FLASH_DISK_SIZE
mkfs.vfat $INSTALL_DIR/.usbdisk.img

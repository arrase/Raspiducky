#!/bin/bash

. ./raspiducky.conf

ETC_DIR=/etc/raspiducky
FLASH_DISK_SIZE=100000 # 100MB
CONFIG_DISK_SIZE=10000 # 10MB

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

dd if=/dev/zero of=$STORAGE_CONFIG bs=1024 count=$CONFIG_DISK_SIZE
mkfs.vfat $STORAGE_CONFIG

[ -d $ETC_DIR ] || sudo mkdir $ETC_DIR
sudo mount $STORAGE_CONFIG $ETC_DIR -o loop,rw

[ -f $ETC_DIR/raspiducky.conf ] || sudo cp raspiducky.conf $ETC_DIR/raspiducky.conf
[ -d $ETC_DIR/payloads-db ] || sudo cp -r payloads $ETC_DIR/payloads-db
[ -d $ETC_DIR/onboot_payload ] || sudo mkdir $ETC_DIR/onboot_payload
echo "$STORAGE_CONFIG   $ETC_DIR    vfat    loop,rw          0       2" | sudo tee --append /etc/fstab
sudo umount $ETC_DIR

# BOOT CONFIG

echo "dtoverlay=dwc2" | sudo tee --append /boot/config.txt
echo "dwc2" | sudo tee --append /etc/modules
echo "libcomposite" | sudo tee --append /etc/modules

cat /etc/rc.local | awk '/exit\ 0/ && c == 0 {c = 0; print "\n/home/pi/hid.sh\nsleep 3\n/home/pi/run_payload.sh\n"}; {print}'| sudo tee /etc/rc.local

# FLASH DRIVE

dd if=/dev/zero of=$STORAGE_FILE bs=1024 count=$FLASH_DISK_SIZE
mkfs.vfat $STORAGE_FILE

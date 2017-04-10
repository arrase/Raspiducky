#!/bin/bash

. etc/raspiducky/raspiducky.conf

BIN_DIR=/usr/bin
ETC_DIR=/etc/raspiducky
FLASH_DISK_SIZE=100000 # 100MB
CONFIG_DISK_SIZE=10000 # 10MB

# DEPENDENCIES
sudo apt update
sudo apt install python-bluez

# EXEC FILES
sudo gcc hid-gadget-test.c -o $BIN_DIR/hid-gadget
sudo cp scripts/hid.sh $BIN_DIR/
sudo cp scripts/run_payload.sh $BIN_DIR/

sudo chmod 777 $BIN_DIR/hid-gadget
sudo chmod 777 $BIN_DIR/hid.sh
sudo chmod 777 $BIN_DIR/run_payload.sh

# FIX BLUETOOTH FROM: https://gist.github.com/arrase/5ed707a3070ef527743d12c971dae6ef
sudo sed 's/bluetooth\/bluetoothd/bluetooth\/bluetoothd\ --compat/' -i /lib/systemd/system/bluetooth.service

# APP CONFIG
[ -d $VAR_DIR ] || sudo mkdir $VAR_DIR

sudo dd if=/dev/zero of=$CONFIG_DISK bs=1024 count=$CONFIG_DISK_SIZE
sudo mkfs.vfat $CONFIG_DISK

[ -d $ETC_DIR ] || sudo mkdir $ETC_DIR
sudo mount $CONFIG_DISK $ETC_DIR -o loop,rw

[ -f $ETC_DIR/raspiducky.conf ] || sudo cp etc/raspiducky/raspiducky.conf $ETC_DIR/raspiducky.conf
[ -d $ETC_DIR/payloads-db ] || sudo cp -r etc/raspiducky/payloads $ETC_DIR/payloads-db
[ -d $ETC_DIR/keyboard_layouts ] || sudo cp -r etc/raspiducky/keyboard_layouts $ETC_DIR/keyboard_layouts
[ -d $ETC_DIR/onboot_payload ] || sudo mkdir $ETC_DIR/onboot_payload

echo "$CONFIG_DISK   $ETC_DIR    vfat    loop,rw          0       2" | sudo tee --append /etc/fstab
sudo cp $ETC_DIR/keyboard_layouts/db/QWERTY-ES_es.py $ETC_DIR/keyboard_layouts/current.py

# BOOT CONFIG
echo "dtoverlay=dwc2" | sudo tee --append /boot/config.txt
echo "dwc2" | sudo tee --append /etc/modules
echo "libcomposite" | sudo tee --append /etc/modules

awk '/exit\ 0/ && c == 0 {c = 0; print "\n/usr/bin/hid.sh\nsleep 3\n/usr/bin/run_payload.sh\n"}; {print}' /etc/rc.local | sudo tee /etc/rc.local

# FLASH DRIVE
sudo dd if=/dev/zero of=$STORAGE_FILE bs=1024 count=$FLASH_DISK_SIZE
sudo mkfs.vfat $STORAGE_FILE

# INIT SCRIPT
sudo cp etc/init.d/raspiduckyd.sh /etc/init.d/raspiduckyd
sudo chmod 777 /etc/init.d/raspiduckyd
sudo update-rc.d raspiduckyd defaults

# INSTALL RASPIDUCKY
cd ducky
sudo python setup.py install


# Raspiducky

Credits to Original Authors:

* Duckberry Pi: Jeff L. (Renegade_R - renegade_r65@hotmail.com)
* DroidDucky by Andrej Budincevic (https://github.com/anbud/DroidDucky)
* hardpass by girst (https://github.com/girst/hardpass)

### Install:

1) Flash the latest Raspbian Jessie image to an SD card

2) Copy all the files (hid-gadget-test.c, duckpi.sh, usleep.c, run_payload.sh, hid.sh) to /home/pi

3) Compile the hid-gadget-test program, this handles moving the text to the Human Interface Device driver:

       gcc hid-gadget-test.c -o hid-gadget-test

4) Compile usleep, this is a basic function which is not natively supported in Raspbian and is used to account for delays in the program:

       make usleep

5) Ensure all files and scripts are executable (chmod 755 <file>)

6) Activate the dwc2 drivers which allows the device to function in host mode when not connected to a PC:

       echo "dtoverlay=dwc2" | sudo tee -a /boot/config.txt

9) Place dwc2 and libcomposite in the modules file to boot with the OS:

       echo "dwc2" | sudo tee /etc/modules
       echo "libcomposite" | sudo tee /etc/modules

10) Copy the following into your /etc/rc.local file.  This allows you to place a "payload.dd" script in the "boot" drive that appears when you plug the SD card into a computer, it will then copy the file and format it for Unix (because Windows machines format the text differently):

       /home/pi/hid.sh
       sleep 3
       /home/pi/run_payload.sh

11) Copy the actual payload into /boot, this directory can also be accessed in Windows by simply placing your micro SD card into a card reader and copying it to the drive that appears.

       cat payloads/open_terminal/open_mint_terminal.dd payloads/backdoor/bind_shell.dd > /boot/payload.dd

12) Place SD card into the Raspberry Pi Zero, plug it into the target host machine via USB cable in the peripheral micro USB port, NOT THE POWER PORT.  A power cord is not required as the Pi Zero will take power directly from the host machine.

13) Watch the script execute on the host machine

### Resources:

* Premade Ducky Scripts: https://github.com/hak5darren/USB-Rubber-Ducky/wiki
* Original USB Rubber Ducky: http://usbrubberducky.com/#!index.md

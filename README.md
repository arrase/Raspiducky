# Raspiducky

A Keyboard emulator like Rubber Ducky build over Raspberry Pi Zero

### Features:

* Keyboard emulation
* USB Flash Drive emulation

### Configuration

* Flash Raspbian 
* Login as pi, use a screen over HDMI and a keyboard over usb port
* Clone the repository
* Run install script

      chmod 777 install.sh
      ./install.sh

* Install a payload

      sudo cat payloads/open_terminal/open_mint_terminal.dd payloads/backdoor/bind_shell.dd > /boot/payload.dd

### Resources:

* Premade Ducky Scripts: https://github.com/hak5darren/USB-Rubber-Ducky/wiki
* Original USB Rubber Ducky: http://usbrubberducky.com/#!index.md

### Credits:

* Duckberry Pi: Jeff L. (Renegade_R - renegade_r65@hotmail.com)
* DroidDucky by Andrej Budincevic (https://github.com/anbud/DroidDucky)
* hardpass by girst (https://github.com/girst/hardpass)

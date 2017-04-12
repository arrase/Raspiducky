# Raspiducky

A Keyboard emulator like Rubber Ducky build over Raspberry Pi Zero W

### Features:

* Keyboard emulation
* USB Flash Drive emulation
* Expose configuration files over USB Flash disk emulation
* Run payloads over bluetooth

### Help

      $ raspiducky.py -h
      
      usage: raspiducky.py [-h] --payload PAYLOAD [--remote] [--address ADDRESS]

        optional arguments:
          -h, --help            show this help message and exit
          --payload PAYLOAD, -p PAYLOAD
                                Path to payload file
          --remote, -r          Run payload on remote device
          --address ADDRESS, -a ADDRESS
                                Remote device address


### Configuration

Read the [wiki](https://github.com/arrase/Raspiducky/wiki) for detailed instructions.

### Resources:

* Premade Ducky Scripts: https://github.com/hak5darren/USB-Rubber-Ducky/wiki
* Original USB Rubber Ducky: http://usbrubberducky.com/#!index.md

### Credits:

* Duckberry Pi: Jeff L. (Renegade_R - renegade_r65@hotmail.com)
* DroidDucky by Andrej Budincevic (https://github.com/anbud/DroidDucky)
* hardpass by girst (https://github.com/girst/hardpass)

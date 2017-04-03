#!/bin/bash

if [ -f /home/pi/config/onboot_payload/payload.dd ]
then
    python /home/pi/raspiducky.py
fi

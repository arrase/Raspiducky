#!/bin/bash

if [ -f /etc/raspiducky/onboot_payload/payload.dd ]
then
    python /home/pi/raspiducky.py --payload /etc/raspiducky/onboot_payload/payload.dd
fi

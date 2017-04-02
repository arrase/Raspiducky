#!/bin/bash

if [ -f /home/pi/config/onboot_payload/payload.dd ]
then
    cat /home/pi/config/onboot_payload/payload.dd > /home/pi/payload.dd
    tr -d '\r' < /home/pi/payload.dd > /home/pi/payload2.dd
    /home/pi/duckpi.sh /home/pi/payload2.dd
fi

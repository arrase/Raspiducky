#!/bin/bash

cat /boot/payload.dd > /home/pi/payload.dd
tr -d '\r' < /home/pi/payload.dd > /home/pi/payload2.dd
/home/pi/duckpi.sh /home/pi/payload2.dd


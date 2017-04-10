#!/bin/bash

if [ -f /etc/raspiducky/onboot_payload/payload.dd ]
then
    /usr/bin/raspiducky.py --payload /etc/raspiducky/onboot_payload/payload.dd
fi

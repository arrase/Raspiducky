#!/bin/bash

. /etc/raspiducky/raspiducky.conf

[ $RUN_AS_DAEMON == "Yes" ] && /usr/bin/raspiducky.py -d start

if [ -f /etc/raspiducky/onboot_payload/payload.dd ]
then
    /usr/bin/raspiducky.py --payload /etc/raspiducky/onboot_payload/payload.dd
fi

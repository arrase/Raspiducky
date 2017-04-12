#!/bin/bash

. /etc/raspiducky/raspiducky.conf

[ $RUN_AS_DAEMON == "Yes" ] && /bin/hciconfig hci0 piscan && /usr/bin/duckyd.py --start

if [ -f /etc/raspiducky/onboot_payload/payload.dd ]
then
    /usr/bin/raspiducky.py --payload /etc/raspiducky/onboot_payload/payload.dd
fi

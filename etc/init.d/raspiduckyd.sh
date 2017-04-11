#!/bin/bash

### BEGIN INIT INFO
# Provides:        RaspiDucky
# Required-Start:  $bluetooth
# Required-Stop:
# Default-Start:   S 2 3 4 5
# Default-Stop:    1
# Short-Description: Start RaspiDucky daemon
### END INIT INFO

PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin

. /lib/lsb/init-functions

. /etc/raspiducky/raspiducky.conf

DAEMON=/usr/bin/raspiducky.py
PID=/var/run/RaspiDucky.pid

test -x $DAEMON || exit 5

[ $RUN_AS_DAEMON == "Yes" ] || exit 0

case $1 in
        start)
                log_daemon_msg "Starting RaspiDucky daemon" "RaspiDucky"
                test -e $PID && log_daemon_msg "RaspiDucky pid exist??" && rm $PID
                /bin/hciconfig hci0 piscan
                exec $DAEMON -d start
                log_end_msg 0
                exit 0
                ;;
        stop)
                log_daemon_msg "Stopping RaspiDucky daemon" "RaspiDucky"
                /bin/hciconfig hci0 noscan
                exec $DAEMON -d stop
                log_end_msg 0
                exit 0
                ;;
        restart)
                log_daemon_msg "Restart RaspiDucky daemon" "RaspiDucky"
                exec $DAEMON -d restart
                exit 0
                ;;
        *)
                echo "Usage: $0 {start|stop|restart}"
                exit 2
                ;;
esac
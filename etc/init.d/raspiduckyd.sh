#!/bin/bash

### BEGIN INIT INFO
# Provides:        RaspiDucky
# Required-Start:  $local_fs $syslog
# Required-Stop:   $local_fs $syslog
# Default-Start:   2 3 4 5
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
                exec $DAEMON --daemon start
                log_end_msg 0
                ;;
        stop)
                log_daemon_msg "Stopping RaspiDucky daemon" "RaspiDucky"
                exec $DAEMON --daemon stop
                log_end_msg 0
                ;;
        restart)
                log_daemon_msg "Restart RaspiDucky daemon" "RaspiDucky"
                $DAEMON --daemon restart
                ;;
        *)
                echo "Usage: $0 {start|stop|restart}"
                exit 2
                ;;
esac
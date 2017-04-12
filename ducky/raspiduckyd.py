#!/usr/bin/python

import argparse

from RaspiDucky.Daemonize import Daemonize

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--start', help='Start Server', action="store_true")
    parser.add_argument('--stop', help='Stop Server', action="store_true")
    parser.add_argument('--restart', help='Restart Server', action="store_true")

    args = parser.parse_args()

    daemon = Daemonize()
    if args.start:
        daemon.start()
    elif args.stop:
        daemon.stop()
    elif args.restart:
        daemon.restart()
    else:
        parser.print_help()

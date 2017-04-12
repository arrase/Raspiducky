#!/usr/bin/python

import argparse

from RaspiDucky.Daemonize import Daemonize

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--daemon', '-d', required=True, choices=['start', 'stop', 'restart'],
                        help='Run as daemon')

    args = parser.parse_args()

    daemon = Daemonize()
    if args.daemon == 'restart':
        daemon.restart()
    elif args.daemon == 'stop':
        daemon.stop()
    else:
        daemon.start()

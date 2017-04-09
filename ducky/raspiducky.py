#!/usr/bin/python

import argparse
import sys

from RaspiDucky.Daemonize import Daemonize
from RaspiDucky.Ducky import Ducky

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--payload', '-p', required=False, help='Path to payload file')
    parser.add_argument('--daemon', '-d', default='start', choices=['start', 'stop', 'restart'], required=False,
                        help='Run as daemon')

    args = parser.parse_args()

    if args.daemon is not None:
        daemon = Daemonize()
        if args.daemon == 'start':
            daemon.start()
        elif args.daemon == 'stop':
            daemon.stop()
        else:
            daemon.restart()
    elif args.payload is not None:
        ducky = Ducky()
        ducky.run(args.payload)
    else:
        parser.print_help()
        sys.exit(1)

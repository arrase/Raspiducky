#!/usr/bin/python

import argparse
import sys

from RaspiDucky.Daemonize import Daemonize
from RaspiDucky.RunPayload import RunPayload

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--payload', '-p', required=False, help='Path to payload file')
    parser.add_argument('--remote', '-r', required=False, help='Run on remote device', action="store_true")
    parser.add_argument('--address', '-a', required=False, help='Remote device address')
    parser.add_argument('--daemon', '-d', choices=['start', 'stop', 'restart'], required=False,
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
        ducky = RunPayload()
        ducky.run(payload=args.payload, remote=args.remote, address=args.address)
    else:
        parser.print_help()
        sys.exit(1)

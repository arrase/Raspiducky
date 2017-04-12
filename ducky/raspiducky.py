#!/usr/bin/python

import argparse

from RaspiDucky.RunPayload import RunPayload

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--payload', '-p', required=True, help='Path to payload file')
    parser.add_argument('--remote', '-r', required=False, help='Run payload on remote device', action="store_true")
    parser.add_argument('--address', '-a', required=False, help='Remote device address')

    args = parser.parse_args()

    ducky = RunPayload()
    ducky.run(payload=args.payload, remote=args.remote, address=args.address)

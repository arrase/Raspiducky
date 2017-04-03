#!/usr/bin/python

import argparse
import subprocess
import time

import keyboard_layouts.current as kb

DEFDELAY = 0
HID_DEV = "/dev/hidg0"
HID_BIN = "./hid-gadget-test"


class Raspiducky:
    def getKBCode(self, char):
        try:
            return kb.KEYBOARD_LAYOUT[char]
        except KeyError:
            return char.lower()

    def exec_code(self, code, code_type="keyboard"):
        p1 = subprocess.Popen(["echo", code], stdout=subprocess.PIPE)
        p2 = subprocess.Popen([HID_BIN, HID_DEV, code_type], stdin=p1.stdout, stdout=subprocess.PIPE)
        p2.communicate()

    def run(self, payload):
        with open(payload) as f:
            for line in f:
                cmd = line.replace('\n', '').replace('\r', '').split(' ', 1)
                # KEYBOARD
                if (cmd[0] == ""):
                    continue  # Discard empty lines
                elif (cmd[0] == "STRING"):
                    for c in cmd[1]:
                        self.exec_code(self.getKBCode(c))
                elif (cmd[0] == "ENTER"):
                    self.exec_code("enter")
                elif (cmd[0] == "DELAY"):
                    time.sleep(float(cmd[1]) / 1000000.0)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--payload', '-p', required=True, help='Path to payload file')
    args = parser.parse_args()

    ducky = Raspiducky()
    ducky.run(args.payload)

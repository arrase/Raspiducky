#!/usr/bin/python

from RaspiDucky.DuckyScript import DuckyScript


class RunPayload:
    script = None

    def __init__(self):
        self.script = DuckyScript()

    def run(self, payload):
        with open(payload) as f:
            for line in f:
                cmd = line.replace('\n', '').replace('\r', '').split(' ', 1)
                self.script.run(cmd)

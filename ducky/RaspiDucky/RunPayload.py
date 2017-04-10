#!/usr/bin/python

from RaspiDucky.DuckyScript import DuckyScript
from RaspiDucky.RFCommClient import RFCommClient


class RunPayload:
    _script = None
    _client = None

    def run(self, payload, remote=False, address=None):
        if remote:
            self._client = RFCommClient()
            self._client.run(payload=payload, address=address)
        else:
            self._script = DuckyScript()
            with open(payload) as f:
                for line in f:
                    cmd = line.replace('\n', '').replace('\r', '').split(' ', 1)
                    self._script.run(cmd)

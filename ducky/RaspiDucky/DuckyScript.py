import subprocess
import sys
from time import sleep

sys.path.append('/etc/raspiducky/keyboard_layouts')

import current as kb


class DuckyScript:
    _def_delay = 0
    _hid_dev = "/dev/hidg0"
    _hid_bin = "/usr/bin/hid-gadget"
    _last_cmd = ""
    _last_string = ""

    def _getKBCode(self, char):
        try:
            return kb.KEYBOARD_LAYOUT[char]
        except KeyError:
            return char.lower()

    def _exec_code(self, code, code_type="keyboard"):
        p1 = subprocess.Popen(["echo", code], stdout=subprocess.PIPE)
        p2 = subprocess.Popen([self._hid_bin, self._hid_dev, code_type], stdin=p1.stdout, stdout=subprocess.PIPE)
        p2.communicate()

    def _parse_cmd(self, cmd):
        if (cmd[0] == "ENTER"):
            return "enter"
        elif cmd[0] in ["WINDOWS", "GUI"]:
            if len(cmd) > 1:
                return "left-meta " + cmd[1].lower()
            else:
                return "left-meta"
        elif cmd[0] in ["MENU", "APP"]:
            return "left-shift f10"
        elif cmd[0] in ["DOWNARROW", "DOWN"]:
            return "down"
        elif cmd[0] in ["LEFTARROW", "LEFT"]:
            return "left"
        elif cmd[0] in ["RIGHTARROW", "RIGHT"]:
            return "right"
        elif cmd[0] in ["UPARROW", "UP"]:
            return "up"
        elif cmd[0] in ["BREAK", "PAUSE"]:
            return "pause"
        elif cmd[0] in ["ESC", "ESCAPE"]:
            return "escape"
        elif (cmd[0] == "PRINTSCREEN"):
            return "print"
        else:
            return cmd[0].lower()

    def run(self, cmd):
        if (cmd[0] == "STRING"):
            self._last_cmd = "STRING"
            self._last_string = cmd[1]
            for c in cmd[1]:
                self._exec_code(self._getKBCode(c))
        elif (cmd[0] == "DELAY"):
            self._last_cmd = "UNS"
            sleep(float(cmd[1]) / 1000000.0)
        elif cmd[0] in ["DEFAULTDELAY", "DEFAULT_DELAY"]:
            self._last_cmd = "UNS"
            self._def_delay = float(cmd[1]) / 1000000.0
        elif (cmd[0] == "REM"):
            self._last_cmd = "REM"
            print(cmd[1])
        elif (cmd[0] == "SHIFT"):
            self._last_cmd = "left-shift " + self._parse_cmd(cmd[1].split(' ', 1))
            self._exec_code(self._last_cmd)
        elif cmd[0] in ["CONTROL", "CTRL"]:
            self._last_cmd = "left-ctrl"
            if len(cmd) > 1:
                self._last_cmd += " " + self._parse_cmd(cmd[1].split(' ', 1))
            self._exec_code(self._last_cmd)
        elif (cmd[0] == "CTRL-SHIFT"):
            self._last_cmd = "left-ctrl left-shift " + self._parse_cmd(cmd[1].split(' ', 1))
            self._exec_code(self._last_cmd)
        elif (cmd[0] == "ALT"):
            self._last_cmd = "left-alt"
            if len(cmd) > 1:
                self._last_cmd = " " + self._parse_cmd(cmd[1].split(' ', 1))
            self._exec_code(self._last_cmd)
        elif (cmd[0] == "ALT-SHIFT"):
            self._last_cmd = "left-shift left-alt"
            self._exec_code(self._last_cmd)
        elif (cmd[0] == "CTRL-ALT"):
            self._last_cmd = "left-ctrl left-alt " + self._parse_cmd(cmd[1].split(' ', 1))
            self._exec_code(self._last_cmd)
        elif (cmd[0] == "CTRL-SHIFT"):
            self._last_cmd = "left-ctrl left-shift " + self._parse_cmd(cmd[1].split(' ', 1))
            self._exec_code(self._last_cmd)
        elif (cmd[0] == "REPEAT"):
            if self._last_cmd not in ["UNS", "REM", ""]:
                for time in xrange(int(cmd[1])):
                    if (self._last_cmd == "STRING"):
                        for c in self._last_string:
                            self._exec_code(self._getKBCode(c))
                    else:
                        self._exec_code(self._last_cmd)
        else:
            self._last_cmd = self._parse_cmd(cmd)
            self._exec_code(self._last_cmd)

        sleep(self._def_delay)

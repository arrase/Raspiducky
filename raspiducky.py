#!/usr/bin/python

import keyboard_layouts.current as kb

DEFDELAY=0
KEYBOARD="/dev/hidg0 keyboard"
MOUSE="/dev/hidg0 mouse"
PAYLOAD="payload2.dd"

def getKBCode (char):
  try:
    return kb.KEYBOARD_LAYOUT[char]
  except KeyError:
    return char.lower()

with open(PAYLOAD) as f:
    for line in f:
        cmd = line.split(' ',1)
        # Discard empty lines
        if (cmd[0] == "\n"):
            continue
        print(cmd)
        if (cmd[0] == "STRING"):
            for c in cmd[1]:
                kbcode = getKBCode(c)
                print (kbcode)


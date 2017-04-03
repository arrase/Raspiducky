#!/usr/bin/python

import keyboard_layouts.current

DEFDELAY=0
KEYBOARD="/dev/hidg0 keyboard"
MOUSE="/dev/hidg0 mouse"
PAYLOAD="payload2.dd"

def getKBCode (char):
  try:
    return KEYBOARD_LAYOUT[char]
  except KeyError:
    return char.lower()

with open(PAYLOAD) as f:
    for line in f:
        pass
        # <do something with line>


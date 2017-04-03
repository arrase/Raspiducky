#!/usr/bin/python

import keyboard_layouts.current

DEFDELAY=0
KEYBOARD="/dev/hidg0 keyboard"
MOUSE="/dev/hidg0 mouse"

def getKBCode (char):
  try:
    return KEYBOARD_LAYOUT[char]
  except KeyError:
    return char.lower()


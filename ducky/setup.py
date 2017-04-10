from distutils.core import setup

setup(
    # Application name:
    name="raspiducky",

    # Version number (initial):
    version="0.1.0",

    # Application author details:
    author="Juan Ezquerro LLanes",
    author_email="arrase@gmail.com",

    # Packages
    packages=["RaspiDucky"],

    # Details
    url="https://github.com/arrase/Raspiducky",

    description="A Keyboard emulator like Rubber Ducky build over Raspberry Pi Zero W",

    data_files=[
        ('/usr/sbin', ['raspiducky.py'])
    ],
    requires=['pybluez']
)

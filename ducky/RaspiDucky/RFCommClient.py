from bluetooth import *

from RaspiDucky.Configuration import Config


class RFCommClient:
    _config = None

    def __init__(self):
        self._config = Config()

    def run(self, payload, address=None):
        if address is None:
            print("No device specified.  Searching all nearby bluetooth devices")

        service_matches = find_service(uuid=self._config.get_uuid(), address=address)

        if len(service_matches) == 0:
            print("Couldn't find the device")
            sys.exit(0)

        first_match = service_matches[0]
        port = first_match["port"]
        name = first_match["name"]
        host = first_match["host"]

        print("connecting to \"%s\" on %s" % (name, host))

        # Create the client socket
        sock = BluetoothSocket(RFCOMM)
        sock.connect((host, port))

        with open(payload) as f:
            for line in f:
                sock.send(struct.pack('!I', len(line)))
                sock.send(line)

        sock.close()

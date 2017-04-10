from bluetooth import *

from RaspiDucky.DuckyScript import DuckyScript


class RFCommServer:
    _server_sock = None
    _uuid = "94f39d29-7d6d-437d-973b-fba39e49d4ee"
    _ducky = None
    _client_sock = None

    def __init__(self):
        self._ducky = DuckyScript()
        self._server_sock = BluetoothSocket(RFCOMM)
        self._server_sock.bind(("", PORT_ANY))
        self._server_sock.listen(1)

    def advertise(self):
        advertise_service(self._server_sock, "RaspiDucky",
                          service_id=self._uuid,
                          service_classes=[self._uuid, SERIAL_PORT_CLASS],
                          profiles=[SERIAL_PORT_PROFILE])

    def run(self):
        self._client_sock, client_info = self._server_sock.accept()

        try:
            while True:
                data = self._client_sock.recv(1024)
                if len(data) == 0: break
                self._ducky.run(data.replace('\n', '').replace('\r', '').split(' ', 1))
        except IOError:
            pass

        self._client_sock.close()

    def stop(self):
        self._server_sock.close()

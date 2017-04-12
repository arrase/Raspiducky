import atexit
import os
import sys
import time
from signal import SIGTERM

from RaspiDucky.RFCommServer import RFCommServer


class Daemonize:
    _pidfile = '/var/run/RaspiDucky.pid'
    _stdin = '/dev/null'
    _stdout = '/dev/null'
    _stderr = '/dev/null'
    _server = None

    def daemonize(self):
        # do the UNIX double-fork magic
        try:
            pid = os.fork()
            if pid > 0:
                # exit first parent
                sys.exit(0)
        except OSError, e:
            sys.exit(1)

        # decouple from parent environment
        os.chdir("/")
        os.setsid()
        os.umask(0)

        # do second fork
        try:
            pid = os.fork()
            if pid > 0:
                # exit from second parent
                sys.exit(0)
        except OSError, e:
            sys.exit(1)

        # redirect standard file descriptors
        sys.stdout.flush()
        sys.stderr.flush()
        si = file(self._stdin, 'r')
        so = file(self._stdout, 'a+')
        se = file(self._stderr, 'a+', 0)
        os.dup2(si.fileno(), sys.stdin.fileno())
        os.dup2(so.fileno(), sys.stdout.fileno())
        os.dup2(se.fileno(), sys.stderr.fileno())

        # write pidfile
        atexit.register(self.delpid)
        pid = str(os.getpid())
        file(self._pidfile, 'w+').write("%s\n" % pid)

    def delpid(self):
        os.remove(self._pidfile)

    def start(self):
        """
        Start the daemon
        """

        # Check for a pidfile to see if the daemon already runs
        try:
            pf = file(self._pidfile, 'r')
            pid = int(pf.read().strip())
            pf.close()
        except IOError:
            pid = None

        if pid:
            sys.exit(1)

        # Start the daemon
        self.daemonize()
        self.run()

    def stop(self):
        """
        Stop the daemon
        """
        # Get the pid from the pidfile
        try:
            pf = file(self._pidfile, 'r')
            pid = int(pf.read().strip())
            pf.close()
        except IOError:
            pid = None

        if not pid:
            return  # not an error in a restart

        # Try killing the daemon process
        try:
            while 1:
                os.kill(pid, SIGTERM)
                time.sleep(0.1)
        except OSError, err:
            err = str(err)
            if err.find("No such process") > 0:
                if os.path.exists(self._pidfile):
                    os.remove(self._pidfile)
            else:
                print
                str(err)
                sys.exit(1)

    def restart(self):
        """
        Restart the daemon
        """
        self.stop()
        self.start()

    def run(self):
        self._server = RFCommServer()
        atexit.register(self._server.stop)
        self._server.advertise()
        while True:
            self._server.run()

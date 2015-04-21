#!/usr/bin/env python

""" Echo server - reply back with the received message. """

import os
import signal
import socket
import sys


def sigusr1_handler(signum, frame):
    print 'signal %s received, exiting ...' % signum
    sys.exit(0)


def setup():
    signal.signal(signal.SIGUSR1, sigusr1_handler)


def runserver():
    sock = socket.socket()
    sock.bind(('0.0.0.0', int(os.environ.get('PORT', '12345'))))
    sock.listen(10)

    while True:
        c, raddr = sock.accept()
        try:
            d = c.recv(4096)
            c.send(d if d else '')
        finally:
            c.close()


if "__main__" == __name__:
    setup()
    runserver()

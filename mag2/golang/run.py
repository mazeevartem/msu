#!/usr/bin/python
import os

for i in range(10):
    x = i / 10.0
    print x
    os.system("iptables -A INPUT -p udp -i lo -m statistic --mode random --probability " + str(x) + " -j DROP")
    os.system("1>out" + str(i) + " ./hello")
    os.system("iptables -D INPUT -p udp -i lo -m statistic --mode random --probability " + str(x) + " -j DROP")

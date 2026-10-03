#!/usr/bin/env python3
"""test/gen_fixtures.py — deterministic generator for test/fixtures/ (stdlib only).

Writes a one-packet pcap: an Ethernet/IPv4/UDP DNS query for example.com from
10.0.0.1 to 10.0.0.2, enough for zeek to produce conn and dns logs.
"""
import os, struct

HERE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fixtures")
os.makedirs(HERE, exist_ok=True)

FIXED_UNIX = 1709294400  # 2024-03-01T12:00:00Z

def dns_query(name):
    q = b"".join(bytes([len(p)]) + p.encode() for p in name.split(".")) + b"\0"
    return struct.pack(">HHHHHH", 0x1234, 0x0100, 1, 0, 0, 0) + q + struct.pack(">HH", 1, 1)

payload = dns_query("example.com")
udp = struct.pack(">HHHH", 40000, 53, 8 + len(payload), 0) + payload
ip = struct.pack(">BBHHHBBH4s4s", 0x45, 0, 20 + len(udp), 1, 0, 64, 17, 0,
                 bytes([10, 0, 0, 1]), bytes([10, 0, 0, 2])) + udp
eth = bytes.fromhex("0011223344550066778899aa") + struct.pack(">H", 0x0800) + ip

header = struct.pack("<IHHiIII", 0xA1B2C3D4, 2, 4, 0, 0, 65535, 1)
record = struct.pack("<IIII", FIXED_UNIX, 0, len(eth), len(eth)) + eth
out = os.path.join(HERE, "dns-query.pcap")
open(out, "wb").write(header + record)
print("wrote", out)

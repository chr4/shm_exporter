# SMA Home Manager 2.0 Prometheus exporter

This program listens for UDP multicast messages as send out by the SMA Home Manager energy meter, and exports the total power in/out (buy/sell) via a `/metrics` Prometheus endpoint

Currently only retrieving the total watts, but can be easily adapted to also individually read out phases (L1, L2, L3)

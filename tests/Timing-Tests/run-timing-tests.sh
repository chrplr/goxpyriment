#!/bin/sh
# Examples of running a series of 4 5-minutes stimulations, generating video, sound and TTL signals.

echo System Info
./Timing-Tests -sysinfo
 read -p "Press Enter for next test (frames) 300s"
./Timing-Tests -test frames -frames-on 1 -frames-off 2 -cycles=6000 > report-frames.txt
read -p "Press Enter for next test (tones) 300s"
./Timing-Tests -test tones -tone-ms 50 -iti-ms 450 -cycles 600   > report-tones.txt
read -p "Press Enter for next test (triggers (dlp-io8)) 300s" 
./Timing-Tests -test trigger -period-ms 100 -duty 10 -duration-s 300 >report-trigger.txt
read -p "Press Enter for next test (av) 300s" 
./Timing-Tests -test av -iti-ms=490 -cycles=600 > report-av.txt
echo Done


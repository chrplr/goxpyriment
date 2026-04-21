#!/bin/sh
# Examples of running a series of 4 5-minutes stimulations, generating video, sound and TTL signals.

# On a prime system, you may want to force the use of the nvidia card:
# __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia

SDL_AUDIODRIVER=alsa
AUDIO_BUFFSIZE=256

echo System Info
./Timing-Tests -sysinfo | tee >| report-sysinfo.txt

read -p "Press Enter for next test (frames) 300s"
./Timing-Tests -test frames -frames-on 1 -frames-off 2 -cycles=6000 | tee >| report-frames.txt

read -p "Press Enter for next test (tones) 300s"
./Timing-Tests -test tones -audio-frames $AUDIO_BUFFSIZE -tone-ms 50 -iti-ms 450 -cycles 600  | tee   >| report-tones.txt

read -p "Press Enter for next test (triggers (dlp-io8)) 300s" 
./Timing-Tests -test trigger -period-ms 100 -duty 10 -duration-s 300 | tee >| report-trigger.txt

read -p "Press Enter for next test (av) 500s" 
./Timing-Tests -test av -audio-frames $AUDIO_BUFFSIZE -frames-on 12 -frames-off 18 -cycles=1000 | tee > report-av.txt

echo Done


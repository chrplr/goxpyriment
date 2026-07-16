Audio language localizer
========================

Spoken sentences in French, and their temporally reversed versions, are played on a fixed onset schedule started by the scanner trigger, while the participant fixates and listens.

To launch the program:

    go run .  -s 1 list.csv


The timing of presentation of the stimuli is controlled by the csv file passed on the command line

```
$ head list.csv

subj,ntrial,nbloc,langue,sent_dur,fname,sent_onset
1,1,1,french,1948,localizer_01A.wav,4000
1,2,1,french,2516,localizer_01B.wav,6448
1,3,1,french,1640,localizer_01C.wav,9464
1,4,2,control,2358,localizer_r_02A.wav,17104
1,5,2,control,3094,localizer_r_02B.wav,19962
1,6,2,control,2139,localizer_r_02C.wav,23556
1,7,3,french,2292,localizer_03A.wav,31695
...
```

If you want to use different stimuli, you just need to replace the french stimuli by your in the `sound_file` folder, and rebuild the program with `go build .`


(c) Christophe Pallier, 2006

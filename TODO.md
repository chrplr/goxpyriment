* improve the movie player for gv format: the gv file should be read progressively fromvthe disk in a goroutine, if possible (lz4 compression
).  
* during video playback (gv or mpeg-1), I would like to sent triggers on a triggerbox device at very specific frames. 
* maybe movies playing should be considered as a kind of stream palying (seee streams.go) and have a parallel API. It would be great if they can return input events. Yet, I thikn we may want the capacitry to pause the movie, rewind it, ... This is not what stresm do. To be discussed.
* add support for some eyetrackers, e.g., eyelink 1000


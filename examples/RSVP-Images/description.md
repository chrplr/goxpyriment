Here is a short description of the experiment of Hebart et al (2023) which we want to replicate:

# Stimuli

The stimuli are images taken from the THINGS object concept and image database (Hebart et al., 2019). A subset of these images is available in the ./images folder


# RSVP Presentation

The stimuli were presented in rapid serial visual presentation mode: in the MEG version of the experiment, each image was displayed for 500ms, followed by a variable fixation period of 1000±200ms (SOA: 1500±200ms). In the fMRI version, the fixation time was fixed at 4s. 

During the fixation period, the background color is mid-gray and the fixation cue is a small black dot at the center of the screen.


# Oddball detection task:

Subjects were instructed to keep their eyes focused on a central fixation cross on the screen while a fast-paced sequence of object images was presented.

They were told to watch out for occasional, artificially-generated "oddball" images (distinct target images that stood out from the natural object photographs).

When an oddball image appeared, subjects had to press a button on a response pad as quickly as possible.

Here, we are going to generate the oddball trials by pixelating a few natural images, in real time. 

# input file

We will not hardcode the timings in the program. Rather, information will be read from a csv file passed on the command-line.

This csv file contains four columns: 'onset','duration','trial_type','file_path'.
- onset and duration are epxressed in seconds.
- `trial_type` contains either 'exp' (normal image) or 'catch' (oddball).
- the file_path is the complete file_path of the images to be displayed (after being pixelated by our program id the trial_type column is not 'exp')


# Results file

The results file must contain all the information from the input file + a reaction_time column, with RT measured from the onset of presentation of the previous image, ir the particpant pressed a button, 'n/a' otherwise.  


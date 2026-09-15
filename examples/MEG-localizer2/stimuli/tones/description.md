For human tonotopy experiments (e.g., fMRI, EEG/MEG, or psychophysics), the central nervous system processes sound frequency on a **logarithmic scale** (the tonotopic map across the basilar membrane and auditory cortex scales exponentially).

To ensure equal spatial or neural spacing across your 5 stimuli, choose frequencies separated by a fixed fractional-octave step rather than linear increments.

### Recommended Range: 200 Hz to 3,200 Hz (1-Octave Steps)

A **1-octave step** doubling sequence spans the full range of human primary auditory cortex (A1) sensitivity cleanly across 4 octaves without running into edge artifacts or low-frequency phase-locking limits.

$$\text{Frequencies: } \mathbf{2 0 0\text{ Hz}}, \mathbf{4 0 0\text{ Hz}}, \mathbf{8 0 0\text{ Hz}}, \mathbf{1 6 0 0\text{ Hz}}, \mathbf{3 2 0 0\text{ Hz}}$$

* **Why this works:** The primary auditory cortex (A1 / Heschl’s gyrus) maps low frequencies laterally and high frequencies medially. A span from 200 Hz to 3.2 kHz cleanly spans this axis, providing robust neural separation across cortical voxels or channels.

---

### Alternative Configurations

Depending on your experimental modality and research goals, adjust the octave spacing:

* **Medium Range (200 Hz – 1,131 Hz | 0.6-Octave Steps):** Focuses on low-to-mid frequency cortical representations, useful if studying pitch center processing or subcortical (auditory brainstem) responses.

$$\mathbf{200\text{ Hz}}, \mathbf{303\text{ Hz}}, \mathbf{459\text{ Hz}}, \mathbf{696\text{ Hz}}, \mathbf{1131\text{ Hz}}$$


* **Wide High-Frequency Range (200 Hz – 5,120 Hz | 1.16-Octave Steps):** Captures the high-frequency extent of Heschl’s gyrus (up to ~5 kHz).

$$\mathbf{200\text{ Hz}}, \mathbf{450\text{ Hz}}, \mathbf{1010\text{ Hz}}, \mathbf{2270\text{ Hz}}, \mathbf{5120\text{ Hz}}$$



---

### Experimental Design Best Practices

1. **Logarithmic Formula:** To calculate custom 5-tone sets, use:

$$f_n = f_{\text{start}} \cdot 2^{(n-1) \cdot \Delta}$$

*(where $n \in \{1, 2, 3, 4, 5\}$ and $\Delta$ is your chosen octave step size).*
2. **Equal Loudness Calibration:** Do not play all frequencies at identical physical sound pressure levels (dBSPL). Adjust output levels using **ISO 226 equal-loudness contours** (or dB SL - sensation level) so 200 Hz and 3200 Hz sound equally loud to participants.
3. **Envelope Windowing:** Apply a 10–20 ms cosine-squared ($\text{cos}^2$) or Tukey ramp at tone onset and offset to eliminate high-frequency acoustic transients (spectral splatter/clicks).

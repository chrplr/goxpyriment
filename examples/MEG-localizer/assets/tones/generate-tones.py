import numpy as np
from scipy.io import wavfile

# Audio parameters
sample_rate = 44100  # Hz
tone_duration = 0.30  # 300 ms
ramp_duration = 0.02  # 20 ms
frequencies = [440, 480, 523, 571, 622, 679, 740, 807, 880]
freq_hlf = [int(x/2) for x in frequencies]
frequencies = freq_hlf

# Calculate sample lengths
num_samples = int(sample_rate * tone_duration)
ramp_samples = int(sample_rate * ramp_duration)

# Create linear fade-in and fade-out envelope
envelope = np.ones(num_samples)
envelope[:ramp_samples] = np.linspace(0, 1, ramp_samples)
envelope[-ramp_samples:] = np.linspace(1, 0, ramp_samples)

# Time array for a single tone
t = np.linspace(0, tone_duration, num_samples, endpoint=False)

# Generate and save each tone separately
for freq in frequencies:
    tone = np.sin(2 * np.pi * freq * t) * envelope
    audio_int16 = np.int16(tone * 32767)

    filename = f"tone_{freq}Hz.wav"
    wavfile.write(filename, sample_rate, audio_int16)
    print(f"Saved: {filename}")

import os
import wave
from google import genai
from google.genai import types

# List of utterances to synthesize
utterances = [
    "[fast] Compute two plus six!",
    "[fast] Compute one plus four!",
    "[fast] Compute seven plus two!",
    "[fast] Compute five plus three!",
    "[fast] Compute four plus one!",
    "[fast] Compute three plus four!",
    "[fast] Compute six plus three!",
    "[fast] Compute four plus four!"
]

# Initialize client
client = genai.Client()

def save_wav(filename, pcm_bytes, sample_rate=24000):
    """Helper function to convert raw PCM audio bytes to a WAV file."""
    with wave.open(filename, "wb") as wf:
        wf.setnchannels(1)      # Mono audio
        wf.setsampwidth(2)     # 16-bit PCM (2 bytes per sample)
        wf.setframerate(sample_rate)
        wf.writeframes(pcm_bytes)

for i, text in enumerate(utterances, start=1):
    output_filename = f"equation_{i}.wav"
    
    # Request audio generation
    response = client.models.generate_content(
        model="gemini-3.1-flash-tts-preview",
        contents=text,
        config=types.GenerateContentConfig(
            response_modalities=["AUDIO"],
            speech_config=types.SpeechConfig(
                voice_config=types.VoiceConfig(
                    prebuilt_voice_config=types.PrebuiltVoiceConfig(
                        voice_name="Kore"  # Prebuilt voices: Kore, Puck, Fenrir, Aoede, etc.
                    )
                )
            )
        )
    )
    
    # Extract raw audio PCM data from candidate parts
    audio_data = None
    for part in response.candidates[0].content.parts:
        if part.inline_data and part.inline_data.mime_type.startswith("audio/"):
            audio_data = part.inline_data.data
            break

    if audio_data:
        save_wav(output_filename, audio_data)
        print(f"Saved: {output_filename}")
    else:
        print(f"Failed to extract audio for utterance {i}")

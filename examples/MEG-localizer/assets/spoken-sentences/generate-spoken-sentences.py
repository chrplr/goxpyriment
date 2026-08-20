import os
import wave
from google import genai
from google.genai import types

# The 32 sentences of the sentence condition, copied from the README.md of
# this example -- keep the two in step: the images in assets/words/ are
# generated from that same list, and a spoken sentence that no longer matches
# its written counterpart would silently break the comparison.
#
# Capitalised and given a full stop so the synthesiser reads them as
# declarative sentences rather than as a list of words, and tagged [fast] like
# the spoken equations so the two auditory conditions are paced alike.
utterances = [
    "[fast] Girls drink fresh water.",
    "[fast] Birds build small nests.",
    "[fast] Cooks fetch fresh lemon.",
    "[fast] Poets write short poems.",
    "[fast] Women teach young girls.",
    "[fast] Kings enter their house.",
    "[fast] Chefs bring clean plate.",
    "[fast] Teams learn these moves.",
    "[fast] Hosts allow every guest.",
    "[fast] Twins react quite badly.",
    "[fast] Girls spend those coins.",
    "[fast] Monks plant these trees.",
    "[fast] Crews build large boats.",
    "[fast] Chefs slice fresh bread.",
    "[fast] Women write short notes.",
    "[fast] Birds drink river water.",
    "[fast] Kings build stone walls.",
    "[fast] Teams train every night.",
    "[fast] Clans share their bread.",
    "[fast] Poets learn these words.",
    "[fast] Cooks clean their table.",
    "[fast] Girls carry heavy boxes.",
    "[fast] Twins climb steep hills.",
    "[fast] Hosts serve sweet cakes.",
    "[fast] These girls enjoy music.",
    "[fast] Those kings order bread.",
    "[fast] Every child needs sleep.",
    "[fast] Young birds leave nests.",
    "[fast] These women study maths.",
    "[fast] Those poets adore music.",
    "[fast] Seven boats cross river.",
    "[fast] Three teens paint doors.",
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
    output_filename = f"sentence_{i:02d}.wav"
    
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

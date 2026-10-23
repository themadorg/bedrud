# Welcome sound fixtures

Short MP3 clips for `welcome_sound_test.go`. They were generated with ffmpeg (libmp3lame), and their reference readings come from ffmpeg's EBU R128 meter (`-af ebur128=peak=sample`).

| File | Source signal | Integrated loudness | Sample peak |
|------|---------------|---------------------|-------------|
| `valid.mp3` | 660 Hz sine at 0.05, mono, 24 kHz, 2 s | −30.0 LUFS | −26.4 dBFS |
| `valid-stereo.mp3` | 660 Hz left and 440 Hz right at 0.05, 44.1 kHz, 2 s | −27.1 LUFS | −26.2 dBFS |
| `too-loud.mp3` | 440 Hz sine at 0.6, mono, 24 kHz, 2 s | −8.5 LUFS | −4.8 dBFS |
| `peaky.mp3` | 660 Hz sine at 0.02 with a 2 ms burst at 0.98, mono, 24 kHz, 2 s | −30.7 LUFS | +0.4 dBFS |
| `silent.mp3` | Silence, mono, 24 kHz, 2 s | below −70 LUFS | −inf |
| `too-long.mp3` | 660 Hz sine at 0.05, mono, 24 kHz, 9 s | −30.0 LUFS | −26.4 dBFS |
| `unsupported-rate.mp3` | 660 Hz sine at 0.05, mono, 22.05 kHz, 32 kbps, 2 s | −30.0 LUFS | −26.3 dBFS |

`unsupported-rate.mp3` is the case go-mp3 v0.3.4 decodes inaccurately: it reads a −18 dBFS peak and −31.1 LUFS instead of the values above, so 22.05 kHz uploads are rejected.

To regenerate a mono clip, run this from this folder (here `valid.mp3`):

```bash
ffmpeg -f lavfi -i "aevalsrc=0.05*sin(2*PI*660*t):s=24000:d=2" -ac 1 -ar 24000 -c:a libmp3lame -b:a 32k valid.mp3
```

The stereo clip uses `aevalsrc=0.05*sin(2*PI*660*t)|0.05*sin(2*PI*440*t):s=44100:d=2` with `-b:a 48k`, and `unsupported-rate.mp3` uses `s=22050` with `-ar 22050`.

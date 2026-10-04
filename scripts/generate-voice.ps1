param(
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '..\assets\voice'),
    # SAPI speaking rate, -10..10. Higher is shorter without raising the pitch.
    [int]$Rate = 10,
    # Silence kept before the first and after the last audible sample.
    [double]$LeadMs = 1,
    [double]$TailMs = 10,
    # Words still audible after this point are time-compressed to fit, keeping
    # their pitch. 0 disables the limit.
    [double]$MaxMs = 75
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Speech
# WSOLA time-scale modification: overlapping 20 ms windows are re-spaced to
# shorten the clip, each one nudged to where it best continues the previous
# one so the pitch is preserved.
Add-Type @'
using System;
public static class TimeScale {
    public static short[] Stretch(short[] input, double factor, int sampleRate) {
        int window = sampleRate / 50, hop = window / 2, search = sampleRate / 250;
        if (input.Length <= window) return input;
        int length = (int)Math.Round(input.Length * factor);
        double[] sum = new double[length + window], weight = new double[length + window];
        int previous = 0;
        for (int frame = 0; frame * hop < length; frame++) {
            int ideal = Math.Min((int)(frame * hop / factor), input.Length - window), best = ideal;
            int natural = previous + hop;
            if (frame > 0 && natural + window <= input.Length) {
                double bestScore = double.NegativeInfinity;
                for (int shift = -search; shift <= search; shift++) {
                    int candidate = ideal + shift;
                    if (candidate < 0 || candidate + window > input.Length) continue;
                    double score = 0;
                    for (int i = 0; i < window; i++) score += (double)input[candidate + i] * input[natural + i];
                    if (score > bestScore) { bestScore = score; best = candidate; }
                }
            }
            for (int i = 0; i < window; i++) {
                double hann = 0.5 - 0.5 * Math.Cos(2 * Math.PI * (i + 0.5) / window);
                sum[frame * hop + i] += input[best + i] * hann;
                weight[frame * hop + i] += hann;
            }
            previous = best;
        }
        short[] output = new short[length];
        for (int i = 0; i < length; i++) output[i] = (short)Math.Round(weight[i] > 1e-3 ? sum[i] / weight[i] : 0);
        return output;
    }
}
'@

if (-not (Test-Path -LiteralPath $OutputDirectory)) {
    New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
}

$synth = [System.Speech.Synthesis.SpeechSynthesizer]::new()
try {
    $synth.Rate = $Rate
    $synth.Volume = 100
    $voices = @($synth.GetInstalledVoices() | Where-Object {
        $_.Enabled -and $_.VoiceInfo.Culture.Name -eq 'en-US'
    })
    if ($voices.Count -eq 0) {
        throw 'No enabled en-US System.Speech voice is installed.'
    }
    $synth.SelectVoice($voices[0].VoiceInfo.Name)
    $voiceName = $synth.Voice.Name

    foreach ($word in @('left', 'right')) {
        $stream = [System.IO.MemoryStream]::new()
        try {
            $synth.SetOutputToWaveStream($stream)
            $synth.Speak($word)
            $synth.SetOutputToNull()
            $wave = $stream.ToArray()

            # Parse the output format and PCM data chunk before trimming.
            if ($wave.Length -lt 44 -or [Text.Encoding]::ASCII.GetString($wave, 0, 4) -ne 'RIFF' -or
                [Text.Encoding]::ASCII.GetString($wave, 8, 4) -ne 'WAVE') {
                throw "Synthesized output for '$word' is not a RIFF/WAVE file."
            }
            $dataOffset = -1
            $dataLength = 0
            $audioFormat = 0
            $channels = 0
            $sampleRate = 0
            $bitsPerSample = 0
            for ($offset = 12; $offset -le $wave.Length - 8;) {
                $chunkName = [Text.Encoding]::ASCII.GetString($wave, $offset, 4)
                $chunkSize = [BitConverter]::ToInt32($wave, $offset + 4)
                $chunkDataOffset = $offset + 8
                if ($chunkSize -lt 0 -or $chunkDataOffset + $chunkSize -gt $wave.Length) {
                    throw "Synthesized WAV for '$word' has an invalid '$chunkName' chunk size."
                }
                if ($chunkName -eq 'fmt ' -and $chunkSize -ge 16) {
                    $audioFormat = [BitConverter]::ToInt16($wave, $chunkDataOffset)
                    $channels = [BitConverter]::ToInt16($wave, $chunkDataOffset + 2)
                    $sampleRate = [BitConverter]::ToInt32($wave, $chunkDataOffset + 4)
                    $bitsPerSample = [BitConverter]::ToInt16($wave, $chunkDataOffset + 14)
                }
                elseif ($chunkName -eq 'data') {
                    $dataOffset = $chunkDataOffset
                    $dataLength = $chunkSize
                }
                $offset += 8 + $chunkSize + ($chunkSize % 2)
            }
            if ($audioFormat -ne 1 -or $channels -ne 1 -or $sampleRate -le 0 -or
                $bitsPerSample -ne 16 -or $dataOffset -lt 0 -or $dataLength -lt 2) {
                throw "Synthesized WAV for '$word' must be mono, 16-bit PCM; got format=$audioFormat channels=$channels rate=$sampleRate bits=$bitsPerSample."
            }

            $samples = [int]($dataLength / 2)
            $peak = 1
            for ($index = 0; $index -lt $samples; $index++) {
                $peak = [Math]::Max($peak, [Math]::Abs([int][BitConverter]::ToInt16($wave, $dataOffset + 2 * $index)))
            }
            # Audible means at least 5% of the peak, so soft breath noise before
            # and after the word does not count towards its length.
            $first = 0
            $last = $samples - 1
            $threshold = $peak * 0.05
            while ($first -lt $samples) {
                $sample = [BitConverter]::ToInt16($wave, $dataOffset + 2 * $first)
                if ([Math]::Abs([int]$sample) -ge $threshold) { break }
                $first++
            }
            while ($last -ge $first) {
                $sample = [BitConverter]::ToInt16($wave, $dataOffset + 2 * $last)
                if ([Math]::Abs([int]$sample) -ge $threshold) { break }
                $last--
            }

            $leadingSamples = [int][Math]::Round($sampleRate * $LeadMs / 1000)
            $trailingSamples = [int][Math]::Round($sampleRate * $TailMs / 1000)
            $start = [Math]::Max(0, $first - $leadingSamples)
            $end = [Math]::Min($samples - 1, $last + $trailingSamples)
            $clip = [int16[]]::new($end - $start + 1)
            for ($index = 0; $index -lt $clip.Length; $index++) {
                $clip[$index] = [BitConverter]::ToInt16($wave, $dataOffset + 2 * ($start + $index))
            }
            $audibleMs = 1000.0 * ($last - $start + 1) / $sampleRate
            if ($MaxMs -gt 0 -and $audibleMs -gt $MaxMs) {
                $clip = [TimeScale]::Stretch($clip, ($MaxMs - 4) / $audibleMs, $sampleRate)
                $peak = 1
                foreach ($sample in $clip) { $peak = [Math]::Max($peak, [Math]::Abs([int]$sample)) }
            }
            $count = $clip.Length
            $trimmedLength = $count * 2

            # Normalise to 90% of full scale, with short fades so the hard trim
            # does not click.
            $gain = 0.9 * 32767 / $peak
            $fadeIn = [Math]::Max(1, $leadingSamples)
            $fadeOut = [Math]::Max(1, $trailingSamples)
            $pcm = [byte[]]::new($trimmedLength)
            for ($index = 0; $index -lt $count; $index++) {
                $value = $clip[$index] * $gain
                $value *= [Math]::Min(1, ($index + 1) / $fadeIn)
                $value *= [Math]::Min(1, ($count - $index) / $fadeOut)
                [BitConverter]::GetBytes([int16][Math]::Round($value)).CopyTo($pcm, 2 * $index)
            }

            $output = [System.IO.MemoryStream]::new()
            $writer = [System.IO.BinaryWriter]::new($output)
            try {
                $writer.Write([Text.Encoding]::ASCII.GetBytes('RIFF'))
                $writer.Write([int](36 + $trimmedLength))
                $writer.Write([Text.Encoding]::ASCII.GetBytes('WAVE'))
                $writer.Write([Text.Encoding]::ASCII.GetBytes('fmt '))
                $writer.Write([int]16)
                $writer.Write([int16]1) # PCM
                $writer.Write([int16]1) # mono
                $writer.Write([int]$sampleRate)
                $writer.Write([int]($sampleRate * 2))
                $writer.Write([int16]2)
                $writer.Write([int16]16)
                $writer.Write([Text.Encoding]::ASCII.GetBytes('data'))
                $writer.Write([int]$trimmedLength)
                $writer.Write($pcm)
                $writer.Flush()
                [System.IO.File]::WriteAllBytes((Join-Path $OutputDirectory "$word.wav"), $output.ToArray())
            }
            finally {
                $writer.Dispose()
                $output.Dispose()
            }
        }
        finally {
            $synth.SetOutputToNull()
            $stream.Dispose()
        }
    }
    Write-Output "Generated left.wav and right.wav with voice '$voiceName' at 22050 Hz, mono PCM, rate $($synth.Rate)."
}
finally {
    $synth.Dispose()
}

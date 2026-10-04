param(
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '..\assets\voice')
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Speech

if (-not (Test-Path -LiteralPath $OutputDirectory)) {
    New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
}

$synth = [System.Speech.Synthesis.SpeechSynthesizer]::new()
try {
    $synth.Rate = 3
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
            $first = 0
            $last = $samples - 1
            $threshold = 350
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

            $leadingSamples = [int][Math]::Round($sampleRate * 0.010)
            $trailingSamples = [int][Math]::Round($sampleRate * 0.030)
            $start = [Math]::Max(0, $first - $leadingSamples)
            $end = [Math]::Min($samples - 1, $last + $trailingSamples)
            $trimmedLength = ($end - $start + 1) * 2

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
                $writer.Write($wave, $dataOffset + 2 * $start, $trimmedLength)
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

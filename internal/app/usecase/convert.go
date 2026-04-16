package usecase

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// convertAudioToWav преобразует аудио данные в нужный формат
// Возвращает преобразованные данные, content-type и аудио кодировку для API
func convertAudioToWav(audioData []byte) ([]byte, string, string, error) {
	format := detectFormat(audioData)

	switch format {
	case "audio/ogg", "audio/mpeg", "audio/wav", "audio/flac", "video/mp4", "video/webm", "video/avi":
		// Универсальный пайплайн: всегда приводим вход к WAV PCM 16kHz mono.
		data, err := convertToPCM16kMono(audioData, format)
		return data, "audio/pcm;rate=16000", "PCM_S16LE", err
	default:
		return nil, "", "", fmt.Errorf("неподдерживаемый формат аудио: используйте голосовые сообщения telegram")
	}
}

// convertToPCM16kMono преобразует входной аудио/видео поток в WAV PCM 16kHz mono через ffmpeg.
func convertToPCM16kMono(data []byte, format string) ([]byte, error) {
	inputPath := "/tmp/smartmeet_input"
	switch format {
	case "audio/ogg":
		inputPath += ".ogg"
	case "audio/mpeg":
		inputPath += ".mp3"
	case "audio/wav":
		inputPath += ".wav"
	case "audio/flac":
		inputPath += ".flac"
	case "video/mp4":
		inputPath += ".mp4"
	case "video/webm":
		inputPath += ".webm"
	case "video/avi":
		inputPath += ".avi"
	default:
		inputPath += ".bin"
	}
	outputPath := "/tmp/smartmeet_output.wav"

	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		return nil, fmt.Errorf("запись входного файла: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	// Используем ffmpeg для конвертации OGG в WAV 16kHz mono
	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath,
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		outputPath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w, output: %s", err, string(out))
	}

	wavData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("чтение выходного файла: %w", err)
	}

	return wavData, nil
}

// detectFormat определяет формат аудио по первым байтам
func detectFormat(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	// OGG формат (Telegram voice messages)
	if bytes.Equal(data[:4], []byte("OggS")) {
		return "audio/ogg"
	}
	// MP3 формат - ID3v2 tag or MPEG frame sync
	if len(data) >= 3 {
		if bytes.Equal(data[:3], []byte("ID3")) {
			return "audio/mpeg"
		}
		if data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
			return "audio/mpeg"
		}
	}
	// WAV формат
	if bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")) {
		return "audio/wav"
	}
	// FLAC формат
	if bytes.Equal(data[:4], []byte("fLaC")) {
		return "audio/flac"
	}
	// MP4/M4A формат (включая video mp4)
	if len(data) >= 8 && bytes.Equal(data[4:8], []byte("ftyp")) {
		return "video/mp4"
	}
	// WebM формат
	if bytes.HasPrefix(data, []byte("\x1a\x45\xdf\xa3")) {
		return "video/webm"
	}
	// AVI формат
	if bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("AVI ")) {
		return "video/avi"
	}

	return ""
}

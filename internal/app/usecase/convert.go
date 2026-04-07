package usecase

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/hajimehoshi/go-mp3"
)

// Параметры кодирования аудио
const (
	sampleRate = 16000 // Частота дискретизации 16 кГц
	channels   = 1     // Mono канал
	bitDepth   = 16    // 16 бит на семпл
)

// convertAudioToWav преобразует аудио данные в нужный формат
// Возвращает преобразованные данные, content-type и аудио кодировку для API
func convertAudioToWav(audioData []byte) ([]byte, string, string, error) {
	format := detectFormat(audioData)

	switch format {
	case "audio/ogg":
		// Telegram voice messages in OGG format - convert to PCM using ffmpeg
		data, err := convertOggToPcm(audioData)
		return data, "audio/pcm;rate=16000", "PCM_S16LE", err
	case "audio/mpeg", "audio/mp3":
		data, err := convertMp3ToWav(audioData)
		return data, "audio/pcm;rate=16000", "PCM_S16LE", err
	case "audio/wav":
		data, err := convertWavToWav(audioData)
		return data, "audio/pcm;rate=16000", "PCM_S16LE", err
	default:
		return nil, "", "", fmt.Errorf("неподдерживаемый формат аудио: используйте голосовые сообщения telegram")
	}
}

// convertOggToPcm преобразует OGG в PCM используя ffmpeg
func convertOggToPcm(data []byte) ([]byte, error) {
	inputPath := "/tmp/smartmeet_input.ogg"
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

	// OGG формат
	if bytes.Equal(data[:4], []byte("OggS")) {
		return "audio/ogg"
	}
	// MP3 формат (MPEG Audio Layer 3)
	if bytes.Equal(data[:3], []byte("\xFF\xFB")) || bytes.Equal(data[:3], []byte("\xFF\xF3")) || bytes.Equal(data[:3], []byte("\xFF\xF2")) {
		return "audio/mpeg"
	}
	// WAV формат
	if bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")) {
		return "audio/wav"
	}
	// FLAC формат
	if bytes.Equal(data[:4], []byte("fLaC")) {
		return "audio/flac"
	}
	// MP4/M4A формат
	if bytes.Equal(data[:4], []byte("ftyp")) {
		return "audio/mp4"
	}

	return ""
}

// convertMp3ToWav преобразует MP3 в WAV
func convertMp3ToWav(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		return nil, fmt.Errorf("создание декодера mp3: %w", err)
	}

	pcmData, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("декодирование mp3: %w", err)
	}

	resampled, err := resampleTo16kHz(pcmData, int(decoder.SampleRate()), sampleRate)
	if err != nil {
		return nil, fmt.Errorf("ресемплинг mp3: %w", err)
	}

	return encodeToWav(resampled)
}

// convertWavToWav преобразует WAV в нужный формат
func convertWavToWav(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	decoder := NewWavDecoder(reader)

	pcmData, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("декодирование wav: %w", err)
	}

	if decoder.SampleRate() != uint32(sampleRate) {
		resampled, err := resampleTo16kHz(pcmData, int(decoder.SampleRate()), sampleRate)
		if err != nil {
			return nil, fmt.Errorf("ресемплинг wav: %w", err)
		}
		return encodeToWav(resampled)
	}

	return encodeToWav(pcmData)
}

// resampleTo16kHz выполняет ресемплинг к частоте 16 кГц
func resampleTo16kHz(data []byte, srcRate, dstRate int) ([]byte, error) {
	if srcRate == dstRate {
		return data, nil
	}

	ratio := float64(dstRate) / float64(srcRate)
	newLen := int(float64(len(data)) * ratio)
	result := make([]byte, newLen)

	var acc float64
	count := 0

	for i := 0; i < newLen; i++ {
		pos := float64(i) / ratio
		idx := int(pos)
		frac := pos - float64(idx)

		if idx+1 < len(data) {
			sample := float64(int(data[idx]))*(1-frac) + float64(int(data[idx+1]))*frac
			acc += sample
		}
		count++

		if count == channels {
			var val int16
			if acc/float64(channels) > 32767 {
				val = 32767
			} else if acc/float64(channels) < -32768 {
				val = -32768
			} else {
				val = int16(acc / float64(channels))
			}
			binary.LittleEndian.PutUint16(result[i*2:], uint16(val))
			acc = 0
			count = 0
		}
	}

	return result, nil
}

// encodeToWav кодирует PCM данные в WAV формат
func encodeToWav(pcmData []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	var riff = []byte("RIFF")
	var wave = []byte("WAVE")
	var fmtChunk = []byte("fmt ")
	var dataChunk = []byte("data")

	binary.Write(buf, binary.LittleEndian, riff)
	binary.Write(buf, binary.LittleEndian, uint32(36+len(pcmData)))
	binary.Write(buf, binary.LittleEndian, wave)
	binary.Write(buf, binary.LittleEndian, fmtChunk)
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(channels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*channels*bitDepth/8))
	binary.Write(buf, binary.LittleEndian, uint16(channels*bitDepth/8))
	binary.Write(buf, binary.LittleEndian, uint16(bitDepth))
	binary.Write(buf, binary.LittleEndian, dataChunk)
	binary.Write(buf, binary.LittleEndian, uint32(len(pcmData)))
	binary.Write(buf, binary.LittleEndian, pcmData)

	return buf.Bytes(), nil
}

// WavDecoder декодирует WAV файлы
type WavDecoder struct {
	r          *bytes.Reader
	sampleRate uint32
}

// NewWavDecoder создает новый декодер WAV
func NewWavDecoder(r *bytes.Reader) *WavDecoder {
	r.Seek(22, os.SEEK_SET)
	var sr uint32
	binary.Read(r, binary.LittleEndian, &sr)
	r.Seek(44, os.SEEK_SET)
	return &WavDecoder{r: r, sampleRate: sr}
}

// Read читает данные из WAV файла
func (w *WavDecoder) Read(p []byte) (int, error) {
	return w.r.Read(p)
}

// SampleRate возвращает частоту дискретизации
func (w *WavDecoder) SampleRate() uint32 {
	return w.sampleRate
}

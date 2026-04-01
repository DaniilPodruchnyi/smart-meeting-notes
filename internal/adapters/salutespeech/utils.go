package salutespeech

import (
	"crypto/rand"
	"fmt"
	"time"
)

func newRqUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Фолбэк без падения процесса, если CSPRNG временно недоступен.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// UUID v4: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint64(b[10])<<40|uint64(b[11])<<32|uint64(b[12])<<24|uint64(b[13])<<16|uint64(b[14])<<8|uint64(b[15]),
	)
}

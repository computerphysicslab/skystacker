package arw

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadMetadata(t *testing.T) {
	order := binary.LittleEndian
	var buf bytes.Buffer
	buf.Write([]byte{'I', 'I'})
	_ = binary.Write(&buf, order, uint16(42))
	_ = binary.Write(&buf, order, uint32(8))
	// Two ASCII values: "SONY" (4 bytes inline) and "ILCE-7M3\0" (9 bytes, overflow).
	_ = binary.Write(&buf, order, uint16(2))
	// Make: tag=0x010F type=ASCII count=4 value inline "SONY".
	_ = binary.Write(&buf, order, uint16(0x010F))
	_ = binary.Write(&buf, order, uint16(2))
	_ = binary.Write(&buf, order, uint32(4))
	buf.WriteString("SONY")
	// Model: tag=0x0110 type=ASCII count=9 value at offset 38.
	_ = binary.Write(&buf, order, uint16(0x0110))
	_ = binary.Write(&buf, order, uint16(2))
	_ = binary.Write(&buf, order, uint32(9))
	_ = binary.Write(&buf, order, uint32(38))
	_ = binary.Write(&buf, order, uint32(0)) // next IFD
	buf.WriteString("ILCE-7M3\x00")
	b := buf.Bytes()
	if !(Decoder{}).Probe(bytes.NewReader(b)) {
		t.Fatal("synthetic ARW not detected by Probe")
	}
	m, err := ReadMetadata(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if m.Make != "SONY" || m.Model != "ILCE-7M3" {
		t.Fatalf("metadata = %+v", m)
	}
}

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type captureTestPacket struct {
	seconds        uint32
	fraction       uint32
	payload        []byte
	originalLength uint32
}

var _ = Describe("Deterministic packet-capture inventory", func() {
	It("inventories little-endian microsecond PCAP without reading packet payloads", func() {
		packets := []captureTestPacket{
			{seconds: 1_700_000_000, fraction: 250_000, payload: []byte{1, 2, 3, 4}, originalLength: 4},
			{seconds: 1_700_000_001, fraction: 500_000, payload: []byte{5, 6}, originalLength: 4},
		}
		path := filepath.Join(GinkgoT().TempDir(), "traffic.pcap")
		Expect(os.WriteFile(path, classicPCAPTestFixture(binary.LittleEndian, false, 64, 1, packets), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "traffic.pcap", evidenceClassification{Modality: "network_or_system_capture"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "pcap"))
		Expect(metadata).To(HaveKeyWithValue("extractor_version", "1.5.0"))
		Expect(metadata).To(HaveKeyWithValue("byte_order", "little"))
		Expect(metadata).To(HaveKeyWithValue("timestamp_precision", "microseconds"))
		Expect(metadata).To(HaveKeyWithValue("version_major", uint16(2)))
		Expect(metadata).To(HaveKeyWithValue("version_minor", uint16(4)))
		Expect(metadata).To(HaveKeyWithValue("snapshot_length_bytes", uint32(64)))
		Expect(metadata).To(HaveKeyWithValue("link_type", uint16(1)))
		Expect(metadata).To(HaveKeyWithValue("link_type_name", "ethernet"))
		Expect(metadata).To(HaveKeyWithValue("packet_count", uint64(2)))
		Expect(metadata).To(HaveKeyWithValue("total_captured_bytes", uint64(6)))
		Expect(metadata).To(HaveKeyWithValue("total_original_bytes", uint64(8)))
		Expect(metadata).To(HaveKeyWithValue("truncated_packet_count", uint64(1)))
		Expect(metadata).To(HaveKeyWithValue("first_timestamp_utc", "2023-11-14T22:13:20.25Z"))
		Expect(metadata).To(HaveKeyWithValue("last_timestamp_utc", "2023-11-14T22:13:21.5Z"))
		Expect(metadata).To(HaveKeyWithValue("packet_payload_bytes_read", 0))
		Expect(metadata).To(HaveKeyWithValue("protocols_decoded", false))
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", true))
	})

	It("supports big-endian nanosecond PCAP and flags extension disagreement", func() {
		packet := captureTestPacket{seconds: 1_700_000_000, fraction: 123, payload: []byte{1}, originalLength: 1}
		path := filepath.Join(GinkgoT().TempDir(), "traffic.pcapng")
		Expect(os.WriteFile(path, classicPCAPTestFixture(binary.BigEndian, true, 128, 101, []captureTestPacket{packet}), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "traffic.pcapng", evidenceClassification{Modality: "network_or_system_capture"})
		Expect(metadata).To(HaveKeyWithValue("format", "pcap"))
		Expect(metadata).To(HaveKeyWithValue("byte_order", "big"))
		Expect(metadata).To(HaveKeyWithValue("timestamp_precision", "nanoseconds"))
		Expect(metadata).To(HaveKeyWithValue("link_type_name", "raw_ip"))
		Expect(metadata).To(HaveKeyWithValue("first_timestamp_utc", "2023-11-14T22:13:20.000000123Z"))
		Expect(warnings).To(ContainElement(ContainSubstring("extension .pcapng contains classic PCAP")))
	})

	It("rejects malformed PCAP timestamp, length, and record boundaries", func() {
		valid := classicPCAPTestFixture(binary.LittleEndian, false, 64, 1, []captureTestPacket{{seconds: 1, payload: []byte{1, 2}, originalLength: 2}})
		badFraction := append([]byte{}, valid...)
		binary.LittleEndian.PutUint32(badFraction[28:32], 1_000_000)
		badLength := append([]byte{}, valid...)
		binary.LittleEndian.PutUint32(badLength[32:36], 65)
		truncated := valid[:len(valid)-1]

		for _, fixture := range []struct {
			name    string
			payload []byte
			warning string
		}{
			{name: "bad-fraction.pcap", payload: badFraction, warning: "timestamp fraction"},
			{name: "bad-length.pcap", payload: badLength, warning: "exceeds snapshot length"},
			{name: "truncated.pcap", payload: truncated, warning: "data exceeds available"},
		} {
			path := filepath.Join(GinkgoT().TempDir(), fixture.name)
			Expect(os.WriteFile(path, fixture.payload, 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, fixture.name, evidenceClassification{Modality: "network_or_system_capture"})
			Expect(metadata).To(BeEmpty())
			Expect(warnings).To(ConsistOf(ContainSubstring(fixture.warning)))
		}
	})

	It("inventories PCAPNG sections, interfaces, packet blocks, and timestamp resolutions", func() {
		order := binary.LittleEndian
		section := pcapngTestSection(order, -1)
		interface0 := pcapngTestInterface(order, 1, 64, nil)
		interface1 := pcapngTestInterface(order, 101, 128, pcapngTestOption(order, 9, []byte{9}))
		timestamp0 := uint64(1_700_000_000)*1_000_000 + 250_000
		timestamp1 := uint64(1_700_000_001)*1_000_000_000 + 123
		capture := append(section, interface0...)
		capture = append(capture, interface1...)
		capture = append(capture, pcapngTestEnhancedPacket(order, 0, timestamp0, []byte{1, 2, 3, 4}, 4)...)
		capture = append(capture, pcapngTestEnhancedPacket(order, 1, timestamp1, []byte{5, 6}, 4)...)
		capture = append(capture, pcapngTestSimplePacket(order, []byte{7}, 3)...)
		path := filepath.Join(GinkgoT().TempDir(), "traffic.pcapng")
		Expect(os.WriteFile(path, capture, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "traffic.pcapng", evidenceClassification{Modality: "network_or_system_capture"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "pcapng"))
		Expect(metadata).To(HaveKeyWithValue("section_count", 1))
		Expect(metadata).To(HaveKeyWithValue("interface_count", 2))
		Expect(metadata).To(HaveKeyWithValue("block_count", 6))
		Expect(metadata).To(HaveKeyWithValue("enhanced_packet_block_count", 2))
		Expect(metadata).To(HaveKeyWithValue("simple_packet_block_count", 1))
		Expect(metadata).To(HaveKeyWithValue("packet_count", uint64(3)))
		Expect(metadata).To(HaveKeyWithValue("timestamp_unavailable_packet_count", uint64(1)))
		Expect(metadata).To(HaveKeyWithValue("truncated_packet_count", uint64(1)))
		Expect(metadata).To(HaveKeyWithValue("first_timestamp_utc", "2023-11-14T22:13:20.25Z"))
		Expect(metadata).To(HaveKeyWithValue("last_timestamp_utc", "2023-11-14T22:13:21.000000123Z"))
		Expect(metadata).To(HaveKeyWithValue("interface_names_retained", false))
		Expect(metadata).To(HaveKeyWithValue("interface_addresses_retained", false))
		Expect(metadata).To(HaveKeyWithValue("packet_payload_bytes_read", 0))
		interfaces := metadata["interfaces"].([]map[string]any)
		Expect(interfaces).To(ConsistOf(
			And(HaveKeyWithValue("interface_id", uint32(0)), HaveKeyWithValue("packet_count", uint64(2)), HaveKeyWithValue("timestamp_resolution_base", 10), HaveKeyWithValue("timestamp_resolution_power", uint8(6))),
			And(HaveKeyWithValue("interface_id", uint32(1)), HaveKeyWithValue("packet_count", uint64(1)), HaveKeyWithValue("timestamp_resolution_base", 10), HaveKeyWithValue("timestamp_resolution_power", uint8(9))),
		))
	})

	It("supports concatenated PCAPNG sections with different byte order", func() {
		little := append(pcapngTestSection(binary.LittleEndian, -1), pcapngTestInterface(binary.LittleEndian, 1, 64, nil)...)
		little = append(little, pcapngTestEnhancedPacket(binary.LittleEndian, 0, 1_700_000_000_000_000, []byte{1}, 1)...)
		bigEndian := append(pcapngTestSection(binary.BigEndian, -1), pcapngTestInterface(binary.BigEndian, 101, 64, nil)...)
		bigEndian = append(bigEndian, pcapngTestEnhancedPacket(binary.BigEndian, 0, 1_700_000_001_000_000, []byte{2}, 1)...)
		path := filepath.Join(GinkgoT().TempDir(), "concatenated.cap")
		Expect(os.WriteFile(path, append(little, bigEndian...), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "concatenated.cap", evidenceClassification{Modality: "network_or_system_capture"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("section_count", 2))
		Expect(metadata).To(HaveKeyWithValue("interface_count", 2))
		Expect(metadata).To(HaveKeyWithValue("packet_count", uint64(2)))
		sections := metadata["sections"].([]map[string]any)
		Expect(sections).To(ConsistOf(
			HaveKeyWithValue("byte_order", "little"),
			HaveKeyWithValue("byte_order", "big"),
		))
	})

	It("flags decryption-secret blocks without reading their contents", func() {
		order := binary.LittleEndian
		capture := append(pcapngTestSection(order, -1), pcapngTestInterface(order, 1, 64, nil)...)
		secretBody := make([]byte, 12)
		order.PutUint32(secretBody[0:4], 0x544c534b)
		order.PutUint32(secretBody[4:8], 4)
		copy(secretBody[8:12], []byte("test"))
		capture = append(capture, pcapngTestBlock(order, pcapngDecryptionSecretsBlock, secretBody)...)
		path := filepath.Join(GinkgoT().TempDir(), "secrets.pcapng")
		Expect(os.WriteFile(path, capture, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "secrets.pcapng", evidenceClassification{Modality: "network_or_system_capture"})
		Expect(metadata).To(HaveKeyWithValue("decryption_secrets_block_count", 1))
		Expect(metadata).To(HaveKeyWithValue("decryption_secrets_retained", false))
		Expect(metadata).To(HaveKeyWithValue("packet_payload_bytes_read", 0))
		Expect(warnings).To(ContainElement(ContainSubstring("secret payloads were not read")))
	})

	It("rejects PCAPNG mismatched trailers, missing interfaces, and oversized blocks", func() {
		order := binary.LittleEndian
		valid := append(pcapngTestSection(order, -1), pcapngTestInterface(order, 1, 64, nil)...)
		badTrailer := append([]byte{}, valid...)
		binary.LittleEndian.PutUint32(badTrailer[len(badTrailer)-4:], 999)
		missingInterface := append(pcapngTestSection(order, -1), pcapngTestEnhancedPacket(order, 0, 1, []byte{1}, 1)...)
		oversizedBlock := append([]byte{}, pcapngTestSection(order, -1)...)
		oversizedBlock = append(oversizedBlock, make([]byte, 12)...)
		binary.LittleEndian.PutUint32(oversizedBlock[len(oversizedBlock)-12:len(oversizedBlock)-8], 1)
		binary.LittleEndian.PutUint32(oversizedBlock[len(oversizedBlock)-8:len(oversizedBlock)-4], maxCaptureBlockBytes+4)

		for _, fixture := range []struct {
			name    string
			payload []byte
			warning string
		}{
			{name: "bad-trailer.pcapng", payload: badTrailer, warning: "leading and trailing block lengths disagree"},
			{name: "missing-interface.pcapng", payload: missingInterface, warning: "missing interface 0"},
			{name: "oversized-block.pcapng", payload: oversizedBlock, warning: "exceeds inventory limit"},
		} {
			path := filepath.Join(GinkgoT().TempDir(), fixture.name)
			Expect(os.WriteFile(path, fixture.payload, 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, fixture.name, evidenceClassification{Modality: "network_or_system_capture"})
			Expect(metadata).To(BeEmpty())
			Expect(warnings).To(ConsistOf(ContainSubstring(fixture.warning)))
		}
	})

	It("bounds interface options and withholds timestamps when resolution metadata was not inspected", func() {
		order := binary.LittleEndian
		largeOptions := make([]byte, maxCaptureOptionsBytes+4)
		capture := append(pcapngTestSection(order, -1), pcapngTestInterfaceRawOptions(order, 1, 64, largeOptions)...)
		capture = append(capture, pcapngTestEnhancedPacket(order, 0, 1_700_000_000_000_000, []byte{1}, 1)...)
		path := filepath.Join(GinkgoT().TempDir(), "large-options.pcapng")
		Expect(os.WriteFile(path, capture, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "large-options.pcapng", evidenceClassification{Modality: "network_or_system_capture"})
		Expect(metadata).To(HaveKeyWithValue("interface_options_complete", false))
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", false))
		Expect(metadata).To(HaveKeyWithValue("timestamp_unavailable_packet_count", uint64(1)))
		Expect(metadata).To(HaveKeyWithValue("timestamp_bounds_available", false))
		Expect(warnings).To(ContainElement(ContainSubstring("options exceed inspection limit")))
	})
})

func classicPCAPTestFixture(order binary.ByteOrder, nanoseconds bool, snapLength uint32, linkType uint32, packets []captureTestPacket) []byte {
	header := make([]byte, 24)
	if order == binary.LittleEndian {
		copy(header[:4], []byte{0xd4, 0xc3, 0xb2, 0xa1})
		if nanoseconds {
			copy(header[:4], []byte{0x4d, 0x3c, 0xb2, 0xa1})
		}
	} else {
		copy(header[:4], []byte{0xa1, 0xb2, 0xc3, 0xd4})
		if nanoseconds {
			copy(header[:4], []byte{0xa1, 0xb2, 0x3c, 0x4d})
		}
	}
	order.PutUint16(header[4:6], 2)
	order.PutUint16(header[6:8], 4)
	order.PutUint32(header[16:20], snapLength)
	order.PutUint32(header[20:24], linkType)
	payload := header
	for _, packet := range packets {
		record := make([]byte, 16)
		order.PutUint32(record[0:4], packet.seconds)
		order.PutUint32(record[4:8], packet.fraction)
		order.PutUint32(record[8:12], uint32(len(packet.payload)))
		order.PutUint32(record[12:16], packet.originalLength)
		payload = append(payload, record...)
		payload = append(payload, packet.payload...)
	}
	return payload
}

func pcapngTestSection(order binary.ByteOrder, declaredLength int64) []byte {
	body := make([]byte, 16)
	order.PutUint32(body[0:4], 0x1a2b3c4d)
	order.PutUint16(body[4:6], 1)
	order.PutUint16(body[6:8], 0)
	order.PutUint64(body[8:16], uint64(declaredLength))
	return pcapngTestBlock(order, pcapngSectionHeaderBlock, body)
}

func pcapngTestInterface(order binary.ByteOrder, linkType uint16, snapLength uint32, options []byte) []byte {
	if len(options) > 0 {
		options = append(options, pcapngTestOption(order, 0, nil)...)
	}
	return pcapngTestInterfaceRawOptions(order, linkType, snapLength, options)
}

func pcapngTestInterfaceRawOptions(order binary.ByteOrder, linkType uint16, snapLength uint32, options []byte) []byte {
	body := make([]byte, 8, 8+len(options))
	order.PutUint16(body[0:2], linkType)
	order.PutUint32(body[4:8], snapLength)
	body = append(body, options...)
	return pcapngTestBlock(order, pcapngInterfaceDescription, body)
}

func pcapngTestEnhancedPacket(order binary.ByteOrder, interfaceID uint32, timestamp uint64, payload []byte, originalLength uint32) []byte {
	padded := append([]byte{}, payload...)
	padded = append(padded, make([]byte, alignCapture32(len(payload))-len(payload))...)
	body := make([]byte, 20, 20+len(padded))
	order.PutUint32(body[0:4], interfaceID)
	order.PutUint32(body[4:8], uint32(timestamp>>32))
	order.PutUint32(body[8:12], uint32(timestamp))
	order.PutUint32(body[12:16], uint32(len(payload)))
	order.PutUint32(body[16:20], originalLength)
	body = append(body, padded...)
	return pcapngTestBlock(order, pcapngEnhancedPacketBlock, body)
}

func pcapngTestSimplePacket(order binary.ByteOrder, payload []byte, originalLength uint32) []byte {
	padded := append([]byte{}, payload...)
	padded = append(padded, make([]byte, alignCapture32(len(payload))-len(payload))...)
	body := make([]byte, 4, 4+len(padded))
	order.PutUint32(body[0:4], originalLength)
	body = append(body, padded...)
	return pcapngTestBlock(order, pcapngSimplePacketBlock, body)
}

func pcapngTestOption(order binary.ByteOrder, optionType uint16, value []byte) []byte {
	option := make([]byte, 4, 4+alignCapture32(len(value)))
	order.PutUint16(option[0:2], optionType)
	order.PutUint16(option[2:4], uint16(len(value)))
	option = append(option, value...)
	option = append(option, make([]byte, alignCapture32(len(value))-len(value))...)
	return option
}

func pcapngTestBlock(order binary.ByteOrder, blockType uint32, body []byte) []byte {
	Expect(len(body) % 4).To(Equal(0))
	blockLength := uint32(12 + len(body))
	block := make([]byte, 8, blockLength)
	order.PutUint32(block[0:4], blockType)
	order.PutUint32(block[4:8], blockLength)
	block = append(block, body...)
	trailer := make([]byte, 4)
	order.PutUint32(trailer, blockLength)
	return append(block, trailer...)
}

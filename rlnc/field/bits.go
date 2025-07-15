package field

// Data conversion utilities over finite fields

// SplitBitsToFieldElements splits a byte slice into field elements each with exactly k bits
// If totalBits is not divisible by k, the remaining bits are discarded
func SplitBitsToFieldElements(data []byte, k int, field Field) []Element {
	totalBits := len(data) * 8
	numElements := totalBits / k // Discard remaining bits

	result := make([]Element, numElements)
	for i := 0; i < numElements; i++ {
		// Extract k bits starting at bit position i*k
		startBit := i * k
		elementBytes := make([]byte, (k+7)/8) // Round up to nearest byte

		for bit := 0; bit < k; bit++ {
			srcBit := startBit + bit
			srcByte := srcBit / 8
			srcOffset := srcBit % 8

			if srcByte < len(data) && (data[srcByte]&(1<<(7-srcOffset))) != 0 {
				dstByte := bit / 8
				dstOffset := bit % 8
				elementBytes[dstByte] |= 1 << (7 - dstOffset)
			}
		}

		result[i] = field.FromBits(elementBytes, k)
	}

	return result
}

// FieldElementsToBytes converts field elements back to bytes
// If totalBits is not divisible by 8, the output slice is rounded up
func FieldElementsToBytes(elements []Element, k int) []byte {
	totalBits := len(elements) * k
	result := make([]byte, (totalBits+7)/8) // Round up to nearest byte

	for i, element := range elements {
		elementBytes := element.Bits(k)
		startBit := i * k

		for bit := 0; bit < k; bit++ {
			srcByte := bit / 8
			srcOffset := bit % 8

			if srcByte < len(elementBytes) && (elementBytes[srcByte]&(1<<(7-srcOffset))) != 0 {
				dstBit := startBit + bit
				dstByte := dstBit / 8
				dstOffset := dstBit % 8
				if dstByte < len(result) {
					result[dstByte] |= 1 << (7 - dstOffset)
				}
			}
		}
	}

	return result
}

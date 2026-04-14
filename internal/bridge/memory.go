package bridge

func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

package util

func IsNumber(str string) bool {
	for _, r := range str {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func IsAscii(str string) bool {
	for _, r := range str {
		if r >= 0x80 {
			return false
		}
	}
	return true
}

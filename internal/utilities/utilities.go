package utilities

func TruncateStrToLength(input string, maxLength int) string {
	runes := []rune(input)
	if len(runes) <= maxLength {
		return input
	}
	if maxLength < 3 {
		return string(runes[:maxLength])
	}
	return string(runes[:maxLength-3]) + "..."
}

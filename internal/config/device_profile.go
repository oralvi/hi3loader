package config

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const (
	deviceHexAlphabet      = "0123456789abcdef"
	deviceUpperHexAlphabet = "0123456789ABCDEF"
	deviceUpperAlphaNum    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func GenerateDeviceProfile() (DeviceProfile, error) {
	return CompleteDeviceProfile(DeviceProfile{})
}

func prefixedRandomString(prefix, alphabet string, suffixLen int) (string, error) {
	suffix, err := randomString(alphabet, suffixLen)
	if err != nil {
		return "", err
	}
	return prefix + suffix, nil
}

func randomString(alphabet string, length int) (string, error) {
	if length <= 0 {
		return "", nil
	}
	if alphabet == "" {
		return "", fmt.Errorf("random alphabet is empty")
	}
	var builder strings.Builder
	builder.Grow(length)
	limit := big.NewInt(int64(len(alphabet)))
	for i := 0; i < length; i++ {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("random string: %w", err)
		}
		builder.WriteByte(alphabet[index.Int64()])
	}
	return builder.String(), nil
}

// OUI prefixes from real Chinese Android phone manufacturers (Xiaomi, OPPO, Vivo, Huawei, Honor, Realme, OnePlus, Samsung CN)
var androidOUITable = [][3]byte{
	{0x00, 0xEC, 0x0A}, // Xiaomi
	{0x04, 0xCF, 0x8C}, // Xiaomi
	{0x28, 0x6C, 0x07}, // Xiaomi
	{0x34, 0x80, 0xB3}, // Xiaomi
	{0x58, 0x44, 0x98}, // Xiaomi
	{0x64, 0xB4, 0x73}, // Xiaomi
	{0x9C, 0x99, 0xA0}, // Xiaomi
	{0xF4, 0x8B, 0x32}, // Xiaomi
	{0x00, 0x1A, 0x11}, // OPPO
	{0x04, 0x8D, 0x38}, // OPPO
	{0x3C, 0xCB, 0x7C}, // OPPO/Realme
	{0xAC, 0xDF, 0xA1}, // OPPO
	{0x00, 0x90, 0x4C}, // Vivo
	{0x28, 0x3A, 0x4D}, // Vivo
	{0x40, 0x31, 0x3C}, // Vivo
	{0x58, 0x2A, 0xF7}, // Vivo
	{0x9C, 0xB7, 0x0D}, // Huawei/Honor
	{0xC4, 0x86, 0xE9}, // Huawei
	{0xD4, 0x6A, 0xA8}, // Huawei
	{0x6C, 0x72, 0xE7}, // Huawei
	{0x40, 0x4E, 0x36}, // Samsung
	{0x8C, 0x71, 0xF8}, // Samsung
	{0xBC, 0x14, 0x01}, // Samsung
}

func randomMACAddress() (string, error) {
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(androidOUITable))))
	if err != nil {
		return "", fmt.Errorf("random mac address: %w", err)
	}
	oui := androidOUITable[idx.Int64()]
	tail := make([]byte, 3)
	if _, err := rand.Read(tail); err != nil {
		return "", fmt.Errorf("random mac address: %w", err)
	}
	raw := [6]byte{oui[0], oui[1], oui[2], tail[0], tail[1], tail[2]}
	parts := make([]string, 0, 6)
	for _, b := range raw {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return strings.Join(parts, ":"), nil
}

func randomIMEI() (string, error) {
	digits := make([]int, 14)
	for i := 0; i < len(digits); i++ {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("random imei: %w", err)
		}
		digits[i] = int(value.Int64())
	}
	checksum := luhnChecksum(digits)
	var builder strings.Builder
	builder.Grow(15)
	for _, digit := range digits {
		builder.WriteByte(byte('0' + digit))
	}
	builder.WriteByte(byte('0' + checksum))
	return builder.String(), nil
}

func luhnChecksum(digits []int) int {
	sum := 0
	double := true
	for i := len(digits) - 1; i >= 0; i-- {
		value := digits[i]
		if double {
			value *= 2
			if value > 9 {
				value -= 9
			}
		}
		sum += value
		double = !double
	}
	return (10 - (sum % 10)) % 10
}

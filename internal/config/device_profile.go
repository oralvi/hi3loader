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
	androidID, err := randomString(deviceHexAlphabet, 16)
	if err != nil {
		return DeviceProfile{}, err
	}
	macAddress, err := randomMACAddress()
	if err != nil {
		return DeviceProfile{}, err
	}
	imei, err := randomIMEI()
	if err != nil {
		return DeviceProfile{}, err
	}
	runtimeUDID, err := randomString(deviceUpperAlphaNum, 36)
	if err != nil {
		return DeviceProfile{}, err
	}
	userProfileUDID, err := prefixedRandomString("XXA", deviceUpperAlphaNum, 34)
	if err != nil {
		return DeviceProfile{}, err
	}
	curBuvid, err := prefixedRandomString("XZA", deviceUpperAlphaNum, 34)
	if err != nil {
		return DeviceProfile{}, err
	}
	return DeviceProfile{
		AndroidID:       androidID,
		MACAddress:      macAddress,
		IMEI:            imei,
		RuntimeUDID:     runtimeUDID,
		UserProfileUDID: userProfileUDID,
		CurBuvid:        curBuvid,
	}, nil
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

func randomMACAddress() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("random mac address: %w", err)
	}
	// locally administered unicast
	raw[0] = (raw[0] & 0xFC) | 0x02
	parts := make([]string, 0, len(raw))
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

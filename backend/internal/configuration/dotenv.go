package configuration

import (
	"bufio"
	"errors"
	"regexp"
	"sort"
	"strings"
)

var variableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var suspiciousSecretNamePattern = regexp.MustCompile(`(?i)(TOKEN|PASSWORD|SECRET|PRIVATE_KEY|API_KEY|ACCESS_KEY|CREDENTIAL)`)

type ImportedVariable struct {
	Name            string `json:"name"`
	Value           string `json:"value"`
	SecretSuggested bool   `json:"secretSuggested"`
}

func ParseDotEnv(text string) ([]ImportedVariable, error) {
	if len(text) > 1<<20 {
		return nil, errors.New(".env input exceeds 1 MiB")
	}
	values := map[string]ImportedVariable{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, found := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !found || !variableNamePattern.MatchString(name) || len(name) > 128 {
			return nil, errors.New("invalid .env entry on line " + itoa(lineNumber))
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if len(value) > 32768 || strings.ContainsRune(value, 0) {
			return nil, errors.New("invalid .env value on line " + itoa(lineNumber))
		}
		values[name] = ImportedVariable{Name: name, Value: value, SecretSuggested: suspiciousSecretNamePattern.MatchString(name)}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]ImportedVariable, 0, len(names))
	for _, name := range names {
		result = append(result, values[name])
	}
	return result, nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

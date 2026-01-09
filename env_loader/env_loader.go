package envloader

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

func LoadFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return errors.New("Line '" + line + "' has invalid format")
		}

		parts[0] = strings.Trim(parts[0], " ")
		parts[1] = strings.Trim(parts[1], " ")

		os.Setenv(parts[0], parts[1])
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func GetEnv(name, def string) string {
	envar := os.Getenv(name)
	if envar == "" {
		return def
	}
	return envar
}

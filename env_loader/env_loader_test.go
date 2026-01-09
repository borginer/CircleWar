package envloader

import (
	"os"
	"testing"
)

func TestEnvLoader(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		keys  []string
		vars  []string
	}{
		{"basic", []string{"YOU=SHUBANA"}, []string{"YOU"}, []string{"SHUBANA"}},
		{"spaces", []string{" BOSHY  =  BLACK  "}, []string{"BOSHY"}, []string{"BLACK"}},
		{"spaces", []string{" BOSHY  =  BLACK  ", "CAPTAIN = TEEMO"}, []string{"BOSHY", "CAPTAIN"}, []string{"BLACK", "TEEMO"}},
	}
	testFileName := "test_envloader.txt"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, _ := os.Create(testFileName)
			defer file.Close()
			for _, line := range test.lines {
				file.WriteString(line + "\n")
			}

			err := LoadFile(testFileName)
			if err != nil {
				t.Errorf("error from LoadEnv: %s", err)
			}

			for i := range len(test.keys) {
				envar := os.Getenv(test.keys[i])
				if envar != test.vars[i] {
					t.Errorf("got %s want %s", envar, test.vars[i])
				}
			}
		})
	}

	os.Remove(testFileName)
}

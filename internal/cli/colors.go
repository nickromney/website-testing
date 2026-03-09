package cli

import (
	"io"
	"os"
	"strconv"
)

const (
	ansiReset         = "\x1b[0m"
	ansiBoldGreen     = "\x1b[1;32m"
	ansiBoldRed       = "\x1b[1;31m"
	ansiGreenBG       = "\x1b[42m"
	ansiRedBG         = "\x1b[41m"
	plainOKLabel      = "[ OK ]"
	plainFailLabel    = "[FAIL]"
	forceColorEnv     = "CLICOLOR_FORCE"
	alternateForceEnv = "FORCE_COLOR"
)

type humanPalette struct {
	enabled bool
}

func newHumanPalette(w io.Writer) humanPalette {
	return humanPalette{enabled: colorEnabled(w)}
}

func (p humanPalette) checkStatus(passed bool) string {
	if passed {
		if !p.enabled {
			return plainOKLabel
		}
		return "[ " + ansiBoldGreen + "OK" + ansiReset + " ]"
	}
	if !p.enabled {
		return plainFailLabel
	}
	return "[" + ansiBoldRed + "FAIL" + ansiReset + "]"
}

func (p humanPalette) summary(totalChecks int, failedChecks int) string {
	text := "OK"
	if failedChecks > 0 {
		text = "FAIL"
	}
	summary := text
	if totalChecks >= 0 {
		passedChecks := totalChecks - failedChecks
		if failedChecks > 0 {
			summary = text + " (" + itoa(failedChecks) + "/" + itoa(totalChecks) + ")"
		} else {
			summary = text + " (" + itoa(passedChecks) + "/" + itoa(totalChecks) + ")"
		}
	}

	if !p.enabled {
		return summary
	}
	if failedChecks > 0 {
		return ansiRedBG + summary + ansiReset
	}
	return ansiGreenBG + summary + ansiReset
}

func colorEnabled(w io.Writer) bool {
	if envForcesColor(forceColorEnv) || envForcesColor(alternateForceEnv) {
		return true
	}
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func envForcesColor(name string) bool {
	value, ok := os.LookupEnv(name)
	return ok && value != "" && value != "0"
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

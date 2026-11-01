package cli

import (
	"fmt"
	"io"
	"strings"
)

func passDoctorCheck(name string, message string) doctorCheck {
	return doctorCheck{Name: name, Status: "pass", Message: message}
}

func warnDoctorCheck(name string, message string) doctorCheck {
	return doctorCheck{Name: name, Status: "warn", Message: message}
}

func failDoctorCheck(name string, message string) doctorCheck {
	return doctorCheck{Name: name, Status: "fail", Message: message}
}

func writeDoctorReport(w io.Writer, checks []doctorCheck) {
	for _, check := range checks {
		// Continuation lines (a tool's stderr tail) are indented under their check.
		message := strings.ReplaceAll(check.Message, "\n", "\n    ")
		_, _ = fmt.Fprintf(w, "[%s] %s: %s\n", strings.ToUpper(check.Status), check.Name, message)
	}
}

func hasDoctorFailures(checks []doctorCheck) bool {
	for _, check := range checks {
		if check.Status == "fail" {
			return true
		}
	}
	return false
}

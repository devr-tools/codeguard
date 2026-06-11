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
		_, _ = fmt.Fprintf(w, "[%s] %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
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

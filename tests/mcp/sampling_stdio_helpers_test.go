package mcp_test

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type stdioSamplingOutcome struct {
	sawSampling bool
	sawFix      bool
}

type samplingMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
}

func startStdioSamplingServer(t *testing.T, cfg string) (*exec.Cmd, io.WriteCloser, io.ReadCloser) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestMCPServeHelperProcess", "--", cfg)
	cmd.Env = append(os.Environ(), "GO_WANT_MCP_HELPER_PROCESS=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	return cmd, stdin, stdout
}

func assertStdioSamplingRoundTrip(t *testing.T, scanner *bufio.Scanner, write func(string)) {
	t.Helper()
	result := make(chan stdioSamplingOutcome, 1)
	go func() {
		result <- collectStdioSamplingOutcome(t, scanner, write)
	}()
	select {
	case out := <-result:
		if !out.sawSampling {
			t.Fatalf("server did not issue sampling/createMessage")
		}
		if !out.sawFix {
			t.Fatalf("propose_fix never returned a result")
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for sampling round trip")
	}
}

func collectStdioSamplingOutcome(t *testing.T, scanner *bufio.Scanner, write func(string)) stdioSamplingOutcome {
	t.Helper()
	var out stdioSamplingOutcome
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		msg, ok := decodeSamplingMessage(line)
		if !ok {
			continue
		}
		if msg.Method == "sampling/createMessage" {
			out.sawSampling = true
			write(samplingResponse(t, msg.ID))
			continue
		}
		if strings.TrimSpace(string(msg.ID)) == `"fix"` {
			out.sawFix = true
			return out
		}
	}
	return out
}

func decodeSamplingMessage(line string) (samplingMessage, bool) {
	var msg samplingMessage
	if json.Unmarshal([]byte(line), &msg) != nil {
		return msg, false
	}
	return msg, true
}

package support

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// catFileStopTimeout bounds how long Close waits for a batch process to exit
// after its stdin is closed before killing it.
const catFileStopTimeout = 5 * time.Second

// errCatFileUnusable marks a response the batch protocol cannot answer
// exactly like `git show` would (non-blob objects, ambiguous names) or a
// broken stream; the caller falls back to `git show` for that read.
var errCatFileUnusable = errors.New("git cat-file batch cannot serve this read")

// catFileBatches owns one `git cat-file --batch` process per target
// directory. Requests on a process are serialised: the protocol is a single
// ordered request/response stream.
type catFileBatches struct {
	baseRef string

	mu     sync.Mutex
	procs  map[string]*catFileBatch
	closed bool
}

func newCatFileBatches(baseRef string) *catFileBatches {
	return &catFileBatches{baseRef: baseRef, procs: map[string]*catFileBatch{}}
}

// fetch reads rel at the base ref through the target's batch process, falling
// back to `git show` when the process cannot be started, has broken, or the
// response is not a plain blob.
func (b *catFileBatches) fetch(ctx context.Context, dir string, rel string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !catFileSpecSafe(rel) {
		return showBaseFile(ctx, dir, b.baseRef, rel)
	}
	proc := b.procFor(ctx, dir)
	if proc == nil {
		return showBaseFile(ctx, dir, b.baseRef, rel)
	}
	data, err := proc.read(b.baseRef + ":./" + rel)
	if errors.Is(err, errCatFileUnusable) {
		return showBaseFile(ctx, dir, b.baseRef, rel)
	}
	return data, err
}

// procFor returns the live batch process for dir, starting it on first use.
// It returns nil when the process cannot be used, so the caller falls back.
func (b *catFileBatches) procFor(ctx context.Context, dir string) *catFileBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	if proc, ok := b.procs[dir]; ok {
		return proc // nil records a failed start: do not retry per file
	}
	proc, err := startCatFileBatch(ctx, dir)
	if err != nil {
		proc = nil
	}
	b.procs[dir] = proc
	return proc
}

func (b *catFileBatches) close() {
	b.mu.Lock()
	procs := b.procs
	b.procs = map[string]*catFileBatch{}
	b.closed = true
	b.mu.Unlock()
	for _, proc := range procs {
		if proc != nil {
			proc.stop()
		}
	}
}

// catFileBatch is one running `git cat-file --batch` process.
type catFileBatch struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	broken bool
}

// startCatFileBatch launches the batch process. Object specs travel over
// stdin, never argv, so no request can be parsed as a git option; the argv is
// fixed apart from the scan target directory. ctx (the scan context) kills
// the process on cancellation.
func startCatFileBatch(ctx context.Context, dir string) (*catFileBatch, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "cat-file", "--batch") //nolint:gosec // fixed git subcommand; dir is a config scan target path; object specs go over stdin
	cmd.WaitDelay = catFileStopTimeout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	return &catFileBatch{cmd: cmd, stdin: stdin, stdout: bufio.NewReaderSize(stdout, 64<<10)}, nil
}

// read requests one object. A missing object yields an error (like `git
// show` failing for an added file); an object larger than maxGitOutputBytes
// is drained and reported as errGitOutputTooLarge, mirroring runGitCapture.
// Each request is bounded by gitCommandTimeout: a stalled process is killed
// and the batch marked broken.
func (p *catFileBatch) read(spec string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.broken {
		return nil, errCatFileUnusable
	}
	watchdog := time.AfterFunc(gitCommandTimeout, p.kill)
	defer watchdog.Stop()

	data, err := p.roundTrip(spec)
	if p.broken {
		p.kill()
	}
	return data, err
}

func (p *catFileBatch) roundTrip(spec string) ([]byte, error) {
	if _, err := io.WriteString(p.stdin, spec+"\n"); err != nil {
		return nil, p.fail()
	}
	header, err := p.stdout.ReadString('\n')
	if err != nil {
		return nil, p.fail()
	}
	header = strings.TrimSuffix(header, "\n")
	if rest, ok := strings.CutPrefix(header, spec+" "); ok {
		switch rest {
		case "missing":
			return nil, fmt.Errorf("git cat-file: %s: object missing at base ref", spec)
		default: // "ambiguous" and any future single-line status
			return nil, errCatFileUnusable
		}
	}
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return nil, p.fail()
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return nil, p.fail()
	}
	if fields[1] != "blob" || size > maxGitOutputBytes {
		// Skip the payload and its trailing newline so the stream stays in
		// sync; nothing is buffered.
		if _, err = io.CopyN(io.Discard, p.stdout, size+1); err != nil {
			return nil, p.fail()
		}
		if fields[1] != "blob" {
			return nil, errCatFileUnusable
		}
		return nil, errGitOutputTooLarge
	}
	data := make([]byte, size)
	if _, err = io.ReadFull(p.stdout, data); err != nil {
		return nil, p.fail()
	}
	if term, termErr := p.stdout.ReadByte(); termErr != nil || term != '\n' {
		return nil, p.fail()
	}
	return data, nil
}

// fail marks the stream unusable; the in-flight read and every later one fall
// back to `git show`.
func (p *catFileBatch) fail() error {
	p.broken = true
	return errCatFileUnusable
}

func (p *catFileBatch) kill() {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

// stop closes stdin (git exits on EOF) and reaps the process, killing it if
// it does not exit within catFileStopTimeout. Taking the request lock waits
// out an in-flight read.
func (p *catFileBatch) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.broken = true
	_ = p.stdin.Close()
	killer := time.AfterFunc(catFileStopTimeout, p.kill)
	_ = p.cmd.Wait()
	killer.Stop()
}

package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

type Request struct {
	Action    string          `json:"action"`
	SessionID string          `json:"session_id"`
	Payload   json.RawMessage `json:"payload"`
}

type ErrorPayload struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type Response struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  *ErrorPayload   `json:"error,omitempty"`
}

type Caller interface {
	Call(ctx context.Context, req Request) (Response, error)
	Ready() bool
	Close() error
}

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
	ready  bool
}

func Start(ctx context.Context, pythonDir string) (*Client, error) {
	pythonBin := os.Getenv("PYTHON")
	if pythonBin == "" {
		pythonBin = "python3"
	}
	cmd := exec.CommandContext(ctx, pythonBin, "main.py")
	cmd.Dir = pythonDir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create worker stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create worker stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create worker stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start worker: %w", err)
	}
	go pipeWorkerStderr(stderr)

	return &Client{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
		ready:  true,
	}, nil
}

func (c *Client) Call(ctx context.Context, req Request) (Response, error) {
	if !c.ready {
		return Response{}, errors.New("worker is not ready")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	line, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("marshal worker request: %w", err)
	}
	if _, err := c.stdin.Write(append(line, '\n')); err != nil {
		return Response{}, fmt.Errorf("write worker request: %w", err)
	}
	type result struct {
		resp Response
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		replyLine, err := c.reader.ReadBytes('\n')
		if err != nil {
			resultCh <- result{err: fmt.Errorf("read worker response: %w", err)}
			return
		}
		var resp Response
		if err := json.Unmarshal(replyLine, &resp); err != nil {
			resultCh <- result{err: fmt.Errorf("decode worker response: %w", err)}
			return
		}
		resultCh <- result{resp: resp}
	}()
	var res result
	select {
	case <-ctx.Done():
		_ = c.Close()
		return Response{}, ctx.Err()
	case res = <-resultCh:
	}
	if res.err != nil {
		return Response{}, res.err
	}
	resp := res.resp
	if resp.Status == "error" && resp.Error != nil {
		return resp, errors.New(resp.Error.Message)
	}
	return resp, nil
}

func (c *Client) Ready() bool {
	return c != nil && c.ready
}

func pipeWorkerStderr(r io.ReadCloser) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	logger := log.New(os.Stderr, "[worker] ", log.LstdFlags|log.Lmicroseconds)
	for scanner.Scan() {
		logger.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		logger.Printf("stderr pipe closed: %v", err)
	}
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.ready = false
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}
